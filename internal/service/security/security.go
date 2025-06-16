package security

import (
	"context"
	jsoniter "github.com/json-iterator/go"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"github.com/unibackend/uniproxy/internal/service/database"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"net"
	"strconv"
	"sync"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const ServiceName = "security"
const clearInterval = 200 * time.Millisecond

type blockInfo struct {
	ApiCall map[string]time.Time
}

type blackListIp struct {
	sync.RWMutex
	m map[int32]*blockInfo
}

type blackListUser struct {
	sync.RWMutex
	m map[common.UserID]*blockInfo
}

type Service interface {
	service.Interface
	Notificator(database.Notification)
}

type securityService struct {
	log      *logger.Logger
	db       database.DB
	measurer metrics.Measurer

	blackLists struct {
		Ip   blackListIp
		User blackListUser
	}
}

type blackListItem struct {
	Operation string `json:"operation,omitempty"`
	Ip        int    `json:"ip,omitempty"`
	UserId    int    `json:"user_id,omitempty"`
	ApiCall   string `json:"api_call"`
	ExpiredAt int    `json:"expired_at"`
}
type securityResponse struct {
	BlackList []blackListItem `json:"blackList"`
}

func New(db database.DB, log *logger.Logger, measurer metrics.Measurer) Service {
	svc := &securityService{
		log:      log,
		db:       db,
		measurer: measurer,
		blackLists: struct {
			Ip   blackListIp
			User blackListUser
		}{
			Ip: blackListIp{
				m: make(map[int32]*blockInfo),
			},
			User: blackListUser{
				m: make(map[common.UserID]*blockInfo),
			},
		},
	}

	if err := svc.InitBlocked(); err != nil {
		log.Errorf("Error init security service: %v", err)
		return svc
	}

	go svc.cleaner()

	return svc
}

func (s *securityService) Notificator(notification database.Notification) {
	notify := blackListItem{}
	err := json.Unmarshal([]byte(notification.Payload), &notify)
	if err != nil {
		s.log.Errorf("Notify unmarshal error: %v", err)
	}

	switch notify.Operation {
	case "INSERT", "UPDATE":
		s.AddBlock(notify)
	case "DELETE":
		s.DeleteBlock(notify)
	}
	s.measurer.Gauge(metrics.ProxyBlacklistCount).Inc()
}

// Get blocked IPs and users from database
func (s *securityService) InitBlocked() error {
	query := common.MapData{
		"apicall": common.EndpointBlackList,
	}

	result, err := s.db.DefaultStatement().Query(context.Background(), s.log, query.Marshal())
	if err != nil {
		return err
	}

	if result.Rows() != nil {
		defer func() {
			if err := result.Rows().Close(); err != nil {
				s.log.Error(err)
			}
		}()
	}

	var jResponse []byte
	response := securityResponse{}
	if result.Rows().Next() {
		if sErr := result.Rows().Scan(&jResponse); sErr != nil {
			return sErr
		}

		jErr := json.Unmarshal(jResponse, &response)
		if jErr != nil {
			return jErr
		}
	}

	for _, item := range response.BlackList {
		s.AddBlock(item)
	}

	if response.BlackList != nil {
		s.measurer.Gauge(metrics.ProxyBlacklistCount).Set(float64(len(response.BlackList)))
	}

	return nil
}

// Cleaner blacklist
func (s *securityService) cleaner() {
	for {
		now := time.Now()

		// IPs blacklist
		for ip, v := range s.blackLists.Ip.m {
			for apiCall, t := range v.ApiCall {
				if t.Before(now) {
					s.blackLists.Ip.Lock()

					delete(s.blackLists.Ip.m[ip].ApiCall, apiCall)
					if len(v.ApiCall) == 0 {
						delete(s.blackLists.Ip.m, ip)
					}

					s.measurer.Gauge(metrics.ProxyBlacklistCount).Dec()
					s.blackLists.Ip.Unlock()
					s.log.Debugf("SECURITY_UNBLOCKED IP %s for apicall %s", int2IP(int(ip)), apiCall)
				}
			}
		}

		// Users blacklist
		for userId, v := range s.blackLists.User.m {
			for apiCall, t := range v.ApiCall {
				if t.Before(now) {
					s.blackLists.User.Lock()

					delete(s.blackLists.User.m[userId].ApiCall, apiCall)
					if len(v.ApiCall) == 0 {
						delete(s.blackLists.User.m, userId)
					}

					s.measurer.Gauge(metrics.ProxyBlacklistCount).Dec()
					s.blackLists.User.Unlock()
					s.log.Debugf("SECURITY_UNBLOCKED UserID %s for apicall %s", strconv.Itoa(int(userId)), apiCall)
				}
			}
		}

		time.Sleep(clearInterval)
	}
}

func (s *securityService) AddBlock(info blackListItem) {
	if info.ApiCall == "" {
		info.ApiCall = "*"
	}

	if info.Ip != 0 {
		s.blackLists.Ip.Lock()
		if _, ok := s.blackLists.Ip.m[int32(info.Ip)]; !ok {
			s.blackLists.Ip.m[int32(info.Ip)] = &blockInfo{
				ApiCall: make(map[string]time.Time),
			}
		}

		s.blackLists.Ip.m[int32(info.Ip)].ApiCall[info.ApiCall] = time.Unix(int64(info.ExpiredAt), 0)
		s.blackLists.Ip.Unlock()
		s.log.Debugf("SECURITY_BLOCKED IP %s for apicall %s until to %s", int2IP(info.Ip), info.ApiCall, strconv.Itoa(info.ExpiredAt))

	} else if info.UserId > 0 {
		s.blackLists.User.Lock()
		if _, ok := s.blackLists.User.m[common.UserID(info.UserId)]; !ok {
			s.blackLists.User.m[common.UserID(info.UserId)] = &blockInfo{
				ApiCall: make(map[string]time.Time),
			}
		}

		s.blackLists.User.m[common.UserID(info.UserId)].ApiCall[info.ApiCall] = time.Unix(int64(info.ExpiredAt), 0)
		s.blackLists.User.Unlock()
		s.log.Debugf("SECURITY_BLOCKED UserID %s for apicall %s until to %s", strconv.Itoa(info.UserId), info.ApiCall, strconv.Itoa(info.ExpiredAt))
	}
}

func (s *securityService) DeleteBlock(info blackListItem) {
	if info.ApiCall == "" {
		info.ApiCall = "*"
	}

	if info.Ip != 0 {
		if _, ok := s.blackLists.Ip.m[int32(info.Ip)]; ok {
			if _, ok := s.blackLists.Ip.m[int32(info.Ip)].ApiCall[info.ApiCall]; ok {
				s.blackLists.Ip.Lock()
				delete(s.blackLists.Ip.m[int32(info.Ip)].ApiCall, info.ApiCall)

				if len(s.blackLists.Ip.m[int32(info.Ip)].ApiCall) == 0 {
					delete(s.blackLists.Ip.m, int32(info.Ip))
				}
				s.blackLists.Ip.Unlock()
				s.log.Debugf("SECURITY_UNBLOCKED IP %s for apicall %s", int2IP(info.Ip), info.ApiCall)
			}
		}

	} else if info.UserId > 0 {
		if _, ok := s.blackLists.User.m[common.UserID(info.UserId)]; ok {
			if _, ok := s.blackLists.User.m[common.UserID(info.UserId)].ApiCall[info.ApiCall]; ok {
				s.blackLists.User.Lock()
				delete(s.blackLists.User.m[common.UserID(info.UserId)].ApiCall, info.ApiCall)

				if len(s.blackLists.User.m[common.UserID(info.UserId)].ApiCall) == 0 {
					delete(s.blackLists.User.m, common.UserID(info.UserId))
				}
				s.blackLists.User.Unlock()
				s.log.Debugf("SECURITY_UNBLOCKED UserID %s for apicall %s", strconv.Itoa(info.UserId), info.ApiCall)
			}
		}
	}
}

func (s *securityService) IsIpBlocked(ip int32, apiCall *string) bool {
	if block, ok := s.blackLists.Ip.m[ip]; ok {
		if apiCall != nil {

			// Search block for specified api_call
			if _, ok := block.ApiCall[*apiCall]; ok {
				// Can be for expired_at
				return true

				// For any api_call
			} else if _, ok := block.ApiCall["*"]; ok {
				return true
			}

			// apiCall is nil, then any api_call can be matched
		} else if _, ok := block.ApiCall["*"]; ok {
			return true
		}
	}

	return false
}

func (s *securityService) IsUserBlocked(userId common.UserID, apiCall *string) bool {
	if block, ok := s.blackLists.User.m[userId]; ok {
		if apiCall != nil {

			// Search block for specified api_call
			if _, ok := block.ApiCall[*apiCall]; ok {
				// Can be for expired_at
				return true

				// For any api_call
			} else if _, ok := block.ApiCall["*"]; ok {
				return true
			}

			// apiCall is nil, then any api_call can be matched
		} else if _, ok := block.ApiCall["*"]; ok {
			return true
		}
	}

	return false
}

func (s *securityService) Do(task common.Task) service.Result {
	return nil
}

func int2IP(ip int) string {
	res := make(net.IP, 4)
	res[0] = byte(ip >> 24)
	res[1] = byte(ip >> 16)
	res[2] = byte(ip >> 8)
	res[3] = byte(ip)
	return res.String()
}
