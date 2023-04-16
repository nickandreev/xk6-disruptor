package disruptors

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/grafana/xk6-disruptor/pkg/utils/process"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeAgentController struct {
	namespace string
	targets   []string
	executor  *process.FakeExecutor
}

func (f *fakeAgentController) Targets() ([]string, error) {
	return f.targets, nil
}

func (f *fakeAgentController) InjectDisruptorAgent() error {
	return nil
}

func (f *fakeAgentController) ExecCommand(cmd []string) error {
	_, err := f.executor.Exec(cmd[0], cmd[1:]...)
	return err
}

func (f *fakeAgentController) Visit(visitor func(string) []string) error {
	for _, t := range f.targets {
		cmd := visitor(t)
		_, err := f.executor.Exec(cmd[0], cmd[1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}

func newPodDisruptorForTesting(ctx context.Context, selector PodSelector, controller AgentController, k8s kubernetes.Kubernetes) PodDisruptor {
	return &podDisruptor{
		ctx:        ctx,
		selector:   selector,
		controller: controller,
		k8s:        k8s,
	}
}

func Test_PodHTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title         string
		selector      PodSelector
		targets       []string
		containerPort uint
		expectedCmd   string
		expectError   bool
		cmdError      error
		fault         HTTPFault
		opts          HTTPDisruptionOptions
		duration      uint
	}{
		{
			title: "Test error 500",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -r 0.1 -e 500",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test error 500 with error body",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -r 0.1 -e 500 -b {\"error\": 500}",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				ErrorBody: "{\"error\": 500}",
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test Average delay",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -a 100 -v 0",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				AverageDelay: 100,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test exclude list",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -x /path1,/path2",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				Exclude: "/path1,/path2",
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test command execution fault",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
			fault:       HTTPFault{},
			opts:        HTTPDisruptionOptions{},
			duration:    60,
		},
		{
			title: "Default container port not found ",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:       []string{"my-app-pod"},
			containerPort: 90,
			expectedCmd:   "xk6-disruptor-agent http -d 60s",
			expectError:   true,
			fault:         HTTPFault{},
			opts:          HTTPDisruptionOptions{},
			duration:      60,
		},
		{
			title: "Container port not found ",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:       []string{"my-app-pod"},
			containerPort: 8081,
			expectedCmd:   "xk6-disruptor-agent http -d 60s",
			expectError:   true,
			fault:         HTTPFault{Port: 8080},
			opts:          HTTPDisruptionOptions{},
			duration:      60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := process.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: tc.selector.Namespace,
				targets:   tc.targets,
				executor:  executor,
			}

			objs := []runtime.Object{}

			for _, target := range tc.targets {
				port := tc.containerPort
				if port == 0 {
					port = 80
				}
				obj := builders.NewPodBuilder(target).
					WithContainerPort(port).
					WithLabels(tc.selector.Select.Labels).
					WithNamespace(tc.selector.Namespace).
					Build()
				objs = append(objs, obj)
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d := newPodDisruptorForTesting(context.TODO(), tc.selector, controller, k)

			err := d.InjectHTTPFaults(tc.fault, tc.duration, tc.opts)

			if tc.expectError && err != nil {
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			cmd := executor.Cmd()
			if !command.AssertCmdEquals(tc.expectedCmd, cmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmd)
			}
		})
	}
}

func Test_PodGrpcPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title         string
		selector      PodSelector
		targets       []string
		containerPort uint
		fault         GrpcFault
		opts          GrpcDisruptionOptions
		duration      uint
		expectedCmd   string
		expectError   bool
		cmdError      error
	}{
		{
			title: "Test error",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},

			fault: GrpcFault{
				ErrorRate:  0.1,
				StatusCode: 14,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -r 0.1 -s 14",
			expectError: false,
			cmdError:    nil,
		},
		{
			title: "Test error with status message",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				ErrorRate:     0.1,
				StatusCode:    14,
				StatusMessage: "internal error",
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -r 0.1 -s 14 -m internal error",
			expectError: false,
			cmdError:    nil,
		},
		{
			title: "Test Average delay",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				AverageDelay: 100,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -a 100 -v 0",
			expectError: false,
			cmdError:    nil,
		},
		{
			title: "Test exclude list",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				Exclude: "service1,service2",
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -x service1,service2",
			expectError: false,
			cmdError:    nil,
		},
		{
			title: "Test command execution fault",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			fault:       GrpcFault{},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
		},
		{
			title: "Default container port not found ",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:       []string{"my-app-pod"},
			containerPort: 90,
			expectedCmd:   "xk6-disruptor-agent http -d 60s",
			expectError:   true,
			fault:         GrpcFault{},
			opts:          GrpcDisruptionOptions{},
			duration:      60,
		},
		{
			title: "Container port not found ",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:       []string{"my-app-pod"},
			containerPort: 8081,
			expectedCmd:   "xk6-disruptor-agent http -d 60s",
			expectError:   true,
			fault:         GrpcFault{Port: 8080},
			opts:          GrpcDisruptionOptions{},
			duration:      60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := process.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: tc.selector.Namespace,
				targets:   tc.targets,
				executor:  executor,
			}

			objs := []runtime.Object{}

			for _, target := range tc.targets {
				port := tc.containerPort
				if port == 0 {
					port = 80
				}
				obj := builders.NewPodBuilder(target).
					WithContainerPort(port).
					WithLabels(tc.selector.Select.Labels).
					WithNamespace(tc.selector.Namespace).
					Build()
				objs = append(objs, obj)
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d := newPodDisruptorForTesting(context.TODO(), tc.selector, controller, k)

			err := d.InjectGrpcFaults(tc.fault, tc.duration, tc.opts)

			if tc.expectError && err != nil {
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			cmd := executor.Cmd()
			if !command.AssertCmdEquals(tc.expectedCmd, cmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmd)
			}
		})
	}
}
