package grpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	pb "github.com/bucher-brothers/openstack-autoscaler/api/protos"
	"github.com/bucher-brothers/openstack-autoscaler/pkg/provider"
)

// OpenStackGrpcServer implements the CloudProvider gRPC service
type OpenStackGrpcServer struct {
	pb.UnimplementedCloudProviderServer
	provider *provider.OpenStackProvider
}

// NewOpenStackGrpcServer creates a new gRPC server
func NewOpenStackGrpcServer(p *provider.OpenStackProvider) *OpenStackGrpcServer {
	return &OpenStackGrpcServer{
		provider: p,
	}
}

// NodeGroups returns all node groups configured for this cloud provider
func (s *OpenStackGrpcServer) NodeGroups(ctx context.Context, req *pb.NodeGroupsRequest) (*pb.NodeGroupsResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroups %v", req)

	nodeGroups := s.provider.GetNodeGroups()
	pbNodeGroups := make([]*pb.NodeGroup, len(nodeGroups))

	for i, ng := range nodeGroups {
		pbNodeGroups[i] = &pb.NodeGroup{
			Id:      ng.ID(),
			MinSize: int32(ng.MinSize()),
			MaxSize: int32(ng.MaxSize()),
			Debug:   fmt.Sprintf("NodeGroup %s: min=%d, max=%d, flavor=%s", ng.ID(), ng.MinSize(), ng.MaxSize(), ng.Config.FlavorName),
		}
	}

	return &pb.NodeGroupsResponse{
		NodeGroups: pbNodeGroups,
	}, nil
}

// NodeGroupForNode returns the node group for the given node
func (s *OpenStackGrpcServer) NodeGroupForNode(ctx context.Context, req *pb.NodeGroupForNodeRequest) (*pb.NodeGroupForNodeResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupForNode %v", req)

	if req.Node == nil {
		return nil, status.Error(codes.InvalidArgument, "node is required")
	}

	ng, err := s.provider.NodeGroupForNode(req.Node.ProviderID)
	if err != nil {
		klog.Errorf("Failed to find node group for node %s: %v", req.Node.ProviderID, err)
		return &pb.NodeGroupForNodeResponse{
			NodeGroup: &pb.NodeGroup{}, // Empty node group means not managed
		}, nil
	}

	if ng == nil {
		return &pb.NodeGroupForNodeResponse{
			NodeGroup: &pb.NodeGroup{}, // Empty node group means not managed
		}, nil
	}

	return &pb.NodeGroupForNodeResponse{
		NodeGroup: &pb.NodeGroup{
			Id:      ng.ID(),
			MinSize: int32(ng.MinSize()),
			MaxSize: int32(ng.MaxSize()),
			Debug:   fmt.Sprintf("NodeGroup %s: min=%d, max=%d, flavor=%s", ng.ID(), ng.MinSize(), ng.MaxSize(), ng.Config.FlavorName),
		},
	}, nil
}

// PricingNodePrice returns pricing for a node (not implemented)
func (s *OpenStackGrpcServer) PricingNodePrice(ctx context.Context, req *pb.PricingNodePriceRequest) (*pb.PricingNodePriceResponse, error) {
	klog.V(4).Infof("gRPC request: PricingNodePrice %v", req)
	return nil, status.Error(codes.Unimplemented, "PricingNodePrice not implemented")
}

// PricingPodPrice returns pricing for a pod (not implemented)
func (s *OpenStackGrpcServer) PricingPodPrice(ctx context.Context, req *pb.PricingPodPriceRequest) (*pb.PricingPodPriceResponse, error) {
	klog.V(4).Infof("gRPC request: PricingPodPrice %v", req)
	return nil, status.Error(codes.Unimplemented, "PricingPodPrice not implemented")
}

// GPULabel returns the label for GPU nodes (not applicable for OpenStack)
func (s *OpenStackGrpcServer) GPULabel(ctx context.Context, req *pb.GPULabelRequest) (*pb.GPULabelResponse, error) {
	klog.V(4).Infof("gRPC request: GPULabel %v", req)
	return &pb.GPULabelResponse{
		Label: "", // No specific GPU label for OpenStack
	}, nil
}

// GetAvailableGPUTypes returns available GPU types (not implemented)
func (s *OpenStackGrpcServer) GetAvailableGPUTypes(ctx context.Context, req *pb.GetAvailableGPUTypesRequest) (*pb.GetAvailableGPUTypesResponse, error) {
	klog.V(4).Infof("gRPC request: GetAvailableGPUTypes %v", req)
	return &pb.GetAvailableGPUTypesResponse{
		GpuTypes: make(map[string]*anypb.Any),
	}, nil
}

// Cleanup cleans up resources before shutdown
func (s *OpenStackGrpcServer) Cleanup(ctx context.Context, req *pb.CleanupRequest) (*pb.CleanupResponse, error) {
	klog.V(4).Infof("gRPC request: Cleanup %v", req)

	err := s.provider.Cleanup()
	if err != nil {
		klog.Errorf("Cleanup failed: %v", err)
		return nil, status.Errorf(codes.Internal, "cleanup failed: %v", err)
	}

	return &pb.CleanupResponse{}, nil
}

// Refresh refreshes the cloud provider state
func (s *OpenStackGrpcServer) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	klog.V(4).Infof("gRPC request: Refresh %v", req)

	err := s.provider.Refresh()
	if err != nil {
		klog.Errorf("Refresh failed: %v", err)
		return nil, status.Errorf(codes.Internal, "refresh failed: %v", err)
	}

	return &pb.RefreshResponse{}, nil
}

// NodeGroupTargetSize returns the current target size of the node group
func (s *OpenStackGrpcServer) NodeGroupTargetSize(ctx context.Context, req *pb.NodeGroupTargetSizeRequest) (*pb.NodeGroupTargetSizeResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupTargetSize %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	size, err := ng.TargetSize()
	if err != nil {
		klog.Errorf("Failed to get target size for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to get target size: %v", err)
	}

	return &pb.NodeGroupTargetSizeResponse{
		TargetSize: int32(size),
	}, nil
}

// NodeGroupIncreaseSize increases the size of the node group
func (s *OpenStackGrpcServer) NodeGroupIncreaseSize(ctx context.Context, req *pb.NodeGroupIncreaseSizeRequest) (*pb.NodeGroupIncreaseSizeResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupIncreaseSize %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	err := ng.IncreaseSize(int(req.Delta))
	if err != nil {
		klog.Errorf("Failed to increase size for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to increase size: %v", err)
	}

	return &pb.NodeGroupIncreaseSizeResponse{}, nil
}

// NodeGroupDeleteNodes deletes nodes from the node group
func (s *OpenStackGrpcServer) NodeGroupDeleteNodes(ctx context.Context, req *pb.NodeGroupDeleteNodesRequest) (*pb.NodeGroupDeleteNodesResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupDeleteNodes %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	// Convert protobuf nodes to Kubernetes nodes
	nodes := make([]*apiv1.Node, len(req.Nodes))
	for i, pbNode := range req.Nodes {
		nodes[i] = &apiv1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pbNode.Name,
				Annotations: pbNode.Annotations,
				Labels:      pbNode.Labels,
			},
			Spec: apiv1.NodeSpec{
				ProviderID: pbNode.ProviderID,
			},
		}
	}

	err := ng.DeleteNodes(nodes)
	if err != nil {
		klog.Errorf("Failed to delete nodes from node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to delete nodes: %v", err)
	}

	return &pb.NodeGroupDeleteNodesResponse{}, nil
}

// NodeGroupDecreaseTargetSize decreases the target size of the node group
func (s *OpenStackGrpcServer) NodeGroupDecreaseTargetSize(ctx context.Context, req *pb.NodeGroupDecreaseTargetSizeRequest) (*pb.NodeGroupDecreaseTargetSizeResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupDecreaseTargetSize %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	err := ng.DecreaseTargetSize(int(req.Delta))
	if err != nil {
		klog.Errorf("Failed to decrease target size for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to decrease target size: %v", err)
	}

	return &pb.NodeGroupDecreaseTargetSizeResponse{}, nil
}

// NodeGroupNodes returns a list of all nodes in the node group
func (s *OpenStackGrpcServer) NodeGroupNodes(ctx context.Context, req *pb.NodeGroupNodesRequest) (*pb.NodeGroupNodesResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupNodes %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	servers, err := ng.Nodes()
	if err != nil {
		klog.Errorf("Failed to get nodes for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to get nodes: %v", err)
	}

	instances := make([]*pb.Instance, len(servers))
	for i, server := range servers {
		instanceState := pb.InstanceStatus_unspecified
		switch server.Status {
		case "ACTIVE":
			instanceState = pb.InstanceStatus_instanceRunning
		case "BUILD":
			instanceState = pb.InstanceStatus_instanceCreating
		case "DELETED", "DELETING":
			instanceState = pb.InstanceStatus_instanceDeleting
		}

		instances[i] = &pb.Instance{
			Id: server.ID,
			Status: &pb.InstanceStatus{
				InstanceState: instanceState,
				ErrorInfo: &pb.InstanceErrorInfo{
					ErrorCode:          "",
					ErrorMessage:       "",
					InstanceErrorClass: 0,
				},
			},
		}
	}

	return &pb.NodeGroupNodesResponse{
		Instances: instances,
	}, nil
}

// NodeGroupTemplateNodeInfo returns template node info for scale-up simulations
func (s *OpenStackGrpcServer) NodeGroupTemplateNodeInfo(ctx context.Context, req *pb.NodeGroupTemplateNodeInfoRequest) (*pb.NodeGroupTemplateNodeInfoResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupTemplateNodeInfo %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	templateNode, err := ng.TemplateNodeInfo()
	if err != nil {
		klog.Errorf("Failed to get template node info for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to get template node info: %v", err)
	}

	nodeBytes, err := templateNode.Marshal()
	if err != nil {
		klog.Errorf("Failed to marshal template node for node group %s: %v", req.Id, err)
		return nil, status.Errorf(codes.Internal, "failed to marshal template node: %v", err)
	}

	return &pb.NodeGroupTemplateNodeInfoResponse{
		NodeBytes: nodeBytes,
	}, nil
}

// NodeGroupGetOptions returns autoscaling options for the node group
func (s *OpenStackGrpcServer) NodeGroupGetOptions(ctx context.Context, req *pb.NodeGroupAutoscalingOptionsRequest) (*pb.NodeGroupAutoscalingOptionsResponse, error) {
	klog.V(4).Infof("gRPC request: NodeGroupGetOptions %v", req)

	ng := s.provider.GetNodeGroup(req.Id)
	if ng == nil {
		return nil, status.Errorf(codes.NotFound, "node group %s not found", req.Id)
	}

	// Use default options - can be extended in the future
	if req.Defaults == nil {
		return nil, status.Error(codes.InvalidArgument, "defaults are required")
	}

	return &pb.NodeGroupAutoscalingOptionsResponse{
		NodeGroupAutoscalingOptions: &pb.NodeGroupAutoscalingOptions{
			ScaleDownUtilizationThreshold:    req.Defaults.ScaleDownUtilizationThreshold,
			ScaleDownGpuUtilizationThreshold: req.Defaults.ScaleDownGpuUtilizationThreshold,
			ScaleDownUnneededDuration:        req.Defaults.ScaleDownUnneededDuration,
			ScaleDownUnreadyDuration:         req.Defaults.ScaleDownUnreadyDuration,
			MaxNodeProvisionDuration:         req.Defaults.MaxNodeProvisionDuration,
			ZeroOrMaxNodeScaling:             req.Defaults.ZeroOrMaxNodeScaling,
			IgnoreDaemonSetsUtilization:      req.Defaults.IgnoreDaemonSetsUtilization,
		},
	}, nil
}
