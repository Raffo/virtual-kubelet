package ecs

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/virtual-kubelet/virtual-kubelet/manager"
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ECSProvider implements the virtual-kubelet provider interface and communicates with the AWS ECS API.
type ECSProvider struct {
	ecsClient       *ecs.ECS //TODO
	resourceManager *manager.ResourceManager
	nodeName        string
	operatingSystem string
	region          string
	securityGroup   string
	subnet          string
	clusterName     string
}

// NewECSProvider creates a new ECS provider.
func NewECSProvider(config string, rm *manager.ResourceManager, nodeName, operatingSystem string) (*ECSProvider, error) {
	var p ECSProvider

	p.resourceManager = rm
	p.operatingSystem = operatingSystem
	p.nodeName = nodeName

	if config != "" {
		f, err := os.Open(config)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := p.loadConfig(f); err != nil {
			return nil, err
		}
	}

	var err error

	c := aws.NewConfig()
	session, err := session.NewSessionWithOptions(session.Options{
		Config:            *c,
		SharedConfigState: session.SharedConfigEnable,
	})

	if err != nil {
		return nil, err
	}

	p.ecsClient = ecs.New(session)

	return &p, nil
}

// CreatePod runs a task on a ECS cluster. It does not create ECS services as pods are per se not exposed.
func (f *ECSProvider) CreatePod(pod *v1.Pod) error {
	log.Println("Called CreatePod.")

	if isDaemonset(pod) {
		log.Printf("skipping create pod %s as it is part of a daemonset\n", pod.Name)
		return nil
	}

	// create task definition
	task := &ecs.RegisterTaskDefinitionInput{
		Family:                  aws.String(pod.Name),
		RequiresCompatibilities: []*string{aws.String("FARGATE")},
		NetworkMode:             aws.String("awsvpc"),
		ContainerDefinitions:    []*ecs.ContainerDefinition{},
		Cpu:                     aws.String("256"), //TODO replace with proper values. Currently only some fixed combinations are possible in fargate mode.
		Memory:                  aws.String("512"),
	}

	// Iterate over the containers to create and start them.
	for _, ctr := range pod.Spec.Containers {
		c := ecs.ContainerDefinition{
			Name:    aws.String(ctr.Name),
			Image:   aws.String(ctr.Image),
			Command: arrayOfStringToArrayOfStringPtr(ctr.Command),
		}
		task.ContainerDefinitions = append(task.ContainerDefinitions, &c)
	}

	// run task

	svc := f.ecsClient

	result, err := svc.RegisterTaskDefinition(task)
	if handleError(err) != nil {
		return err
	}

	input := &ecs.CreateServiceInput{
		Cluster:        aws.String(f.clusterName),
		ServiceName:    aws.String(pod.Name),
		DesiredCount:   aws.Int64(1),
		TaskDefinition: aws.String(pod.Name),
		LaunchType:     aws.String("FARGATE"),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				Subnets:        []*string{aws.String(f.subnet)},
				SecurityGroups: []*string{aws.String(f.securityGroup)},
				AssignPublicIp: aws.String("ENABLED"),
			},
		},
	}

	_, err = svc.CreateService(input)

	if handleError(err) != nil {
		return err
	}

	pod.Name = pod.Name + "foo"

	log.Println(result)
	return nil
}

// UpdatePod takes a Kubernetes Pod and updates it within the provider.
func (f *ECSProvider) UpdatePod(pod *v1.Pod) error {
	return nil
}

// DeletePod takes a Kubernetes Pod and deletes it from the provider.
func (f *ECSProvider) DeletePod(pod *v1.Pod) error {
	return nil
}

// GetPod retrieves a pod by name from the provider (can be cached).
func (f *ECSProvider) GetPod(namespace, name string) (*v1.Pod, error) {
	log.Printf("Called GetPod: %s\n", name)

	out, err := f.ecsClient.DescribeServices(&ecs.DescribeServicesInput{
		Cluster: aws.String(f.clusterName),
		Services: []*string{
			aws.String(name),
		},
	})

	if err != nil {
		return nil, err
	}

	log.Println(out)

	return nil, nil
}

// GetPodStatus retrieves the status of a pod by name from the provider.
func (f *ECSProvider) GetPodStatus(namespace, name string) (*v1.PodStatus, error) {
	return nil, nil
}

// GetPods retrieves a list of all pods running on the provider (can be cached).
func (f *ECSProvider) GetPods() ([]*v1.Pod, error) {
	log.Println("Called GetPods.")
	fmt.Println(f.clusterName)
	input := &ecs.ListServicesInput{
		Cluster: aws.String(f.clusterName),
	}

	out, err := f.ecsClient.ListServices(input)
	if err != nil {
		return nil, err
	}

	pods := make([]*v1.Pod, 0) // currently only returning the arns
	for _, s := range out.ServiceArns {
		pods = append(pods, &v1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: *s,
		}})
	}

	log.Println(pods)

	return pods, nil
}

// Capacity returns a resource list with the capacity constraints of the provider.
func (f *ECSProvider) Capacity() v1.ResourceList {
	// TODO: These are totally fake
	return v1.ResourceList{
		"cpu":    resource.MustParse("20"),
		"memory": resource.MustParse("100Gi"),
		"pods":   resource.MustParse("20"),
	}
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), which is polled periodically to update the node status
// within Kuberentes.
func (f *ECSProvider) NodeConditions() []v1.NodeCondition {
	// TODO: These are totally fake
	return []v1.NodeCondition{
		{
			Type:               "Ready",
			Status:             v1.ConditionTrue,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletReady",
			Message:            "kubelet is ready.",
		},
		{
			Type:               "OutOfDisk",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientDisk",
			Message:            "kubelet has sufficient disk space available",
		},
		{
			Type:               "MemoryPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               "DiskPressure",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               "NetworkUnavailable",
			Status:             v1.ConditionFalse,
			LastHeartbeatTime:  metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             "RouteCreated",
			Message:            "RouteController created a route",
		},
	}
}

// OperatingSystem returns the operating system the provider is for.
func (f *ECSProvider) OperatingSystem() string {
	return providers.OperatingSystemLinux
}

func arrayOfStringToArrayOfStringPtr(array []string) []*string {
	converted := make([]*string, len(array))
	for _, a := range array {
		converted = append(converted, aws.String(a))
	}
	return converted
}

func handleError(err error) error {
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				log.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				log.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				log.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				log.Println(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			case ecs.ErrCodeUnsupportedFeatureException:
				log.Println(ecs.ErrCodeUnsupportedFeatureException, aerr.Error())
			case ecs.ErrCodePlatformUnknownException:
				log.Println(ecs.ErrCodePlatformUnknownException, aerr.Error())
			case ecs.ErrCodePlatformTaskDefinitionIncompatibilityException:
				log.Println(ecs.ErrCodePlatformTaskDefinitionIncompatibilityException, aerr.Error())
			case ecs.ErrCodeAccessDeniedException:
				log.Println(ecs.ErrCodeAccessDeniedException, aerr.Error())
			case ecs.ErrCodeBlockedException:
				log.Println(ecs.ErrCodeBlockedException, aerr.Error())
			default:
				log.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Println(err.Error())
		}
		return err
	}
	return nil
}

func isDaemonset(pod *v1.Pod) bool {
	if len(pod.OwnerReferences) > 0 {
		if pod.OwnerReferences[0].Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
