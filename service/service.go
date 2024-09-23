package service

import (
	"context"
	"fmt"
	"net"
	"path"

	// etcd
    // eclient "go.etcd.io/etcd/client/v3"
    // "go.etcd.io/etcd/client/v3/naming/endpoints"
	// eresolver "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "grpc-etcd/proto"
)

type calcServer struct {
	pb.UnimplementedCalcServiceServer
}

func (s *calcServer) Calculate(ctx context.Context, req *pb.CalcRequest) (*pb.CalcResponse, error) {
	a := req.GetA()
	b := req.GetB()
	opt := req.GetOpt()
	fmt.Printf("Server calculate: %v %v %v\n", a, opt, b)

	var result float64
	switch opt {
	case "+":
	  result = a + b
	case "-":
	  result = a - b
	case "*":
	  result = a * b
	case "/":
	  result = a / b
	default:
	  // 如果运算符不合法，返回一个错误
	  return nil, fmt.Errorf("invalid operator: %s", opt)
	}

	return &pb.CalcResponse{ Result: result }, nil
}

// func (s *calcServer) mustEmbedUnimplementedCalcServiceServer() {}

func newCalcServer() *calcServer { return new(calcServer) }

func StartServer(port int, rootPath string) error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		fmt.Printf("Server listen error: %v\n", err)
		return err
	}

	certFile := path.Join(rootPath, "key", "server", "server.crt")
	keyFile := path.Join(rootPath, "key", "server", "server.key")
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		fmt.Printf("Create credentials failed: %v\n", err)
		return err
	}

	serverOption := []grpc.ServerOption{grpc.Creds(creds)}
	grpcServer := grpc.NewServer(serverOption...)
	pb.RegisterCalcServiceServer(grpcServer, newCalcServer())
	fmt.Printf("Start grpc server, Listen %v port\n", port)

	if err := grpcServer.Serve(listen); err != nil {
		fmt.Printf("GRPC server error: %v\n", err)
		return err
	}

	return nil
}
