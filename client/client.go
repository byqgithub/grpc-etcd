package client

import (
	"context"
	"fmt"
	"path"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "grpc-etcd/proto"
)

func Calcuate(ctx context.Context, client pb.CalcServiceClient, a, b float64, opt string) (float64, error) {
	reqInfo := pb.CalcRequest{A: a, B: b, Opt: opt}
	response, err := client.Calculate(ctx, &reqInfo)
	if err != nil {
		fmt.Printf("Calcuate error: %v\n", err)
	} else {
		fmt.Printf("response string: %v\n", response.String())
		return response.GetResult(), nil
	}

	return 0, fmt.Errorf("Calcuate error: %v", err)
}

func InitClient(ctx context.Context, rootPath string, addr string) (*grpc.ClientConn, pb.CalcServiceClient) {
	certFile := path.Join(rootPath, "key", "client", "ca.crt")
	creds, err := credentials.NewClientTLSFromFile(certFile, "")
	if err != nil {
		fmt.Printf("Client create creds error: %v\n", err)
		return nil, nil
	}

	options := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	conn, err := grpc.Dial(addr, options...)
	if err != nil {
		fmt.Printf("Client create conn error: %v\n", err)
		return nil, nil
	}

	return conn, pb.NewCalcServiceClient(conn)
}

func CloseClient(client *grpc.ClientConn) {
	client.Close()

	grpc.WithResolvers()
}
