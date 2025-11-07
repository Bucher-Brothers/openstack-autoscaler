package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog/v2"

	pb "github.com/bucher-brothers/openstack-autoscaler/api/protos"
	"github.com/bucher-brothers/openstack-autoscaler/pkg/config"
	grpcserver "github.com/bucher-brothers/openstack-autoscaler/pkg/grpc"
	"github.com/bucher-brothers/openstack-autoscaler/pkg/provider"
)

var (
	// Server flags
	address = flag.String("address", ":8086", "The address to expose the grpc service")
	keyCert = flag.String("key-cert", "", "The path to the certificate key file. Empty string for insecure communication")
	cert    = flag.String("cert", "", "The path to the certificate file. Empty string for insecure communication")
	cacert  = flag.String("ca-cert", "", "The path to the ca certificate file. Empty string for insecure communication")

	// OpenStack configuration flags
	configFile = flag.String("config", "", "Path to the OpenStack autoscaler configuration file")

	// OpenStack cloud flags (can be used instead of config file)
	authURL     = flag.String("auth-url", "", "OpenStack authentication URL (OS_AUTH_URL)")
	username    = flag.String("username", "", "OpenStack username (OS_USERNAME)")
	password    = flag.String("password", "", "OpenStack password (OS_PASSWORD)")
	projectName = flag.String("project-name", "", "OpenStack project name (OS_PROJECT_NAME)")
	projectID   = flag.String("project-id", "", "OpenStack project ID (OS_PROJECT_ID)")
	region      = flag.String("region", "", "OpenStack region (OS_REGION_NAME)")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("Starting OpenStack Autoscaler gRPC Server")

	// Load configuration
	cfg, err := loadConfiguration()
	if err != nil {
		klog.Fatalf("Failed to load configuration: %v", err)
	}

	// Create OpenStack provider
	openstackProvider, err := provider.NewOpenStackProvider(cfg)
	if err != nil {
		klog.Fatalf("Failed to create OpenStack provider: %v", err)
	}

	// Validate configuration
	if err := openstackProvider.ValidateConfiguration(context.Background()); err != nil {
		klog.Fatalf("Configuration validation failed: %v", err)
	}

	// Create gRPC server
	grpcServer := createGRPCServer()

	// Create and register our service
	service := grpcserver.NewOpenStackGrpcServer(openstackProvider)
	pb.RegisterCloudProviderServer(grpcServer, service)

	// Start server
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		klog.Fatalf("Failed to listen: %v", err)
	}

	klog.Infof("OpenStack Autoscaler gRPC server listening on %s", *address)
	if err := grpcServer.Serve(listener); err != nil {
		klog.Fatalf("Failed to serve: %v", err)
	}
}

func loadConfiguration() (*config.Config, error) {
	if *configFile != "" {
		klog.Infof("Loading configuration from file: %s", *configFile)
		return config.LoadConfig(*configFile)
	}

	// Load from environment variables or command line flags
	cloudConfig := loadCloudConfig()

	cfg := &config.Config{
		Cloud: *cloudConfig,
	}

	return cfg, nil
}

func loadCloudConfig() *config.CloudConfig {
	// Load from environment variables first
	cloudCfg := config.LoadConfigFromEnv()

	// Override with command line flags if provided
	if *authURL != "" {
		cloudCfg.AuthURL = *authURL
	}
	if *username != "" {
		cloudCfg.Username = *username
	}
	if *password != "" {
		cloudCfg.Password = *password
	}
	if *projectName != "" {
		cloudCfg.ProjectName = *projectName
	}
	if *projectID != "" {
		cloudCfg.ProjectID = *projectID
	}
	if *region != "" {
		cloudCfg.Region = *region
	}

	return cloudCfg
}

func createGRPCServer() *grpc.Server {
	var serverOpts []grpc.ServerOption

	// Check if TLS certificates are provided
	if *keyCert != "" && *cert != "" && *cacert != "" {
		klog.Info("Setting up TLS for gRPC server")

		// Load server certificate
		certificate, err := tls.LoadX509KeyPair(*cert, *keyCert)
		if err != nil {
			klog.Fatalf("Failed to load certificate files: %v", err)
		}

		// Load CA certificate
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(*cacert)
		if err != nil {
			klog.Fatalf("Failed to read CA certificate: %v", err)
		}

		if !certPool.AppendCertsFromPEM(ca) {
			klog.Fatal("Failed to append CA certificate")
		}

		// Configure TLS
		tlsConfig := &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		}

		transportCreds := credentials.NewTLS(tlsConfig)
		serverOpts = append(serverOpts, grpc.Creds(transportCreds))

		klog.Info("TLS configured successfully")
	} else {
		klog.Warning("No TLS certificates provided, using insecure connection")
	}

	return grpc.NewServer(serverOpts...)
}
