package grpc

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"os"
)

// Service
type Service interface {
	ProcessData(context.Context, *message) (*message, error)
}

type grpcService struct{}

func registerService(s *grpc.Server, svc Service) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "UniBackend",
		HandlerType: (*Service)(nil),
		Methods:     []grpc.MethodDesc{
			/*{
				MethodName: "ProcessData",
				Handler:    func(srv any, ctx context.Context, dec func(any) error, interceptor UnaryServerInterceptor) (any, error),
			},*/
		},
		Streams: []grpc.StreamDesc{},
	}, svc)
}

func (srv *grpcService) ProcessData(ctx context.Context, msg *message) (*message, error) {
	// Прочитать содержимое файла с данными protobuf
	data, err := os.ReadFile("yourdata.pb")
	if err != nil {
		fmt.Printf("Failed to read file: %v\n", err)
		return nil, err
	}

	// Десериализовать данные protobuf в структуру
	if err := proto.Unmarshal(data, msg); err != nil {
		fmt.Printf("Failed to unmarshal data: %v\n", err)
		return nil, err
	}

	// Теперь вы можете использовать msg как обычную Go-структуру.
	fmt.Printf("Unmarshaled Message: %+v\n", msg)
	//return srv.ProcessData(ctx, in)
	return msg, nil
}

type message map[string]interface{}

func (m message) Reset() {
	m = nil
}
func (m message) String() string {
	if data, err := json.Marshal(m); err == nil {
		return string(data)
	}
	return ""
}
func (m message) ProtoMessage() {}
