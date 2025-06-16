package geoip

import (
	"github.com/oschwald/geoip2-golang"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/service"
	"net"
)

const ServiceName = "geoIp"

type Service interface {
	service.Interface
	Record(ip net.IP) (*geoip2.City, error)
}

type geoIpService struct {
	db *geoip2.Reader
}

func New(path string) Service {
	var db *geoip2.Reader
	var err error

	if path != "" {
		db, err = geoip2.Open(path)
		if err != nil {
			panic(err)
		}
	}

	return &geoIpService{
		db: db,
	}
}

func (s *geoIpService) Close() error {
	return s.db.Close()
}

func (s *geoIpService) Record(ip net.IP) (*geoip2.City, error) {
	record, err := s.db.City(ip)
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (s *geoIpService) Do(task common.Task) service.Result {
	return nil
}
