package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/koku-web3/txbuilder-tron/api"
	"github.com/koku-web3/txbuilder-tron/internal/config"
	"github.com/koku-web3/txbuilder-tron/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configPath := flag.String("config", config.DEFAULT_PATH, "path to config file")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := config.InitLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}

	log := config.GetLogger()
	log.Info("starting txbuilder-tron service",
		"chain_id", config.GetConfig().Chain.ChainID,
		"rpc_url", config.GetConfig().Chain.RPCURL,
		"grpc_host", config.GetConfig().GRPC.Host,
		"grpc_port", config.GetConfig().GRPC.Port,
	)

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d",
		config.GetConfig().GRPC.Host,
		config.GetConfig().GRPC.Port))
	if err != nil {
		log.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	api.RegisterTxBuilderServer(grpcServer, service.NewTxBuilderService())

	reflection.Register(grpcServer)

	go func() {
		log.Info("gRPC server listening", "addr", lis.Addr().String())
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down gRPC server")
	grpcServer.GracefulStop()
}
