package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/client/cache"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/watch"
	"time"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/api"
	"reflect"
	"fmt"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
)

func newTestWatchRestClient() *RESTClient {
	return NewRESTClient(&store.FakeResourceStore{})
}

func getResourceWatcher(client cache.Getter, resource string) watch.Interface {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource, api.NamespaceAll, fields.ParseSelectorOrDie(""))
	// ask for watcher data
	timemoutseconds := int64(10)

	options := api.ListOptions{
		ResourceVersion: "0",
		// We want to avoid situations of hanging watchers. Stop any wachers that do not
		// receive any events within the timeout window.
		TimeoutSeconds: &timemoutseconds,
	}

	w, _ := listerWatcher.Watch(options)
	return w
}

func emitEvent(client *RESTClient, resource string, test eventTest) {
	switch resource {
		case ccapi.Pods:
			client.EmitPodWatchEvent(test.event, test.item.(*api.Pod))
		case ccapi.Services:
			client.EmitServiceWatchEvent(test.event, test.item.(*api.Service))
		case ccapi.ReplicationControllers:
			client.EmitReplicationControllerWatchEvent(test.event, test.item.(*api.ReplicationController))
		case ccapi.PersistentVolumes:
			client.EmitPersistentVolumeWatchEvent(test.event, test.item.(*api.PersistentVolume))
		case ccapi.Nodes:
			client.EmitNodeWatchEvent(test.event, test.item.(*api.Node))
		case ccapi.PersistentVolumeClaims:
			client.EmitPersistentVolumeClaimWatchEvent(test.event, test.item.(*api.PersistentVolumeClaim))
		default:
			fmt.Printf("Unsupported resource %s", resource)
			// TODO(jchaloup): log the error
	}
}

type eventTest struct{
	event watch.EventType
	item interface{}
}

func testWatch(tests []eventTest, resource string, t *testing.T) {

	client := newTestWatchRestClient()
	w := getResourceWatcher(client, resource)

	t.Logf("Emitting first two events")
	emitEvent(client, resource, tests[0])
	emitEvent(client, resource, tests[1])
	// wait for a while so both events are in one byte stream
	time.Sleep(10*time.Millisecond)
	sync := make(chan struct{})

	// retrieve all events one by one in the same order
	go func() {
		for _, test := range tests {
			t.Logf("Waiting for event")
			event, ok := <-w.ResultChan()
			if !ok {
				t.Errorf("Unexpected watch close")
			}
			t.Logf("Event received")
			if event.Type != test.event {
				t.Errorf("Expected event type %q, got %q", test.event, event.Type)
			}
			if !reflect.DeepEqual(test.item, event.Object) {
				t.Errorf("unexpected object: expected: %#v\n actual: %#v", test.item, event.Object)
			}
		}
		sync<- struct{}{}
	}()

	// send remaining events
	t.Logf("Emitting remaining events")
	for _, test := range tests[2:] {
		time.Sleep(10*time.Millisecond)
		emitEvent(client, resource, test)
		t.Logf("Event emitted")
	}

	// wait for all events
	<-sync
	close(sync)
	client.Close()
}

func TestWatchPods(t *testing.T) {

	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "pod1", Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: pod,
		},
		{
			event: watch.Added,
			item: pod,
		},
		{
			event: watch.Modified,
			item: pod,
		},
		{
			event: watch.Deleted,
			item: pod,
		},
	}

	testWatch(tests, ccapi.Pods, t)
}

func TestWatchServices(t *testing.T) {

	service := &api.Service{
		ObjectMeta: api.ObjectMeta{Name: "service1", Namespace: "test", ResourceVersion: "12"},
		Spec: api.ServiceSpec{
			SessionAffinity: "None",
			Type:            api.ServiceTypeClusterIP,
		},
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: service,
		},
		{
			event: watch.Added,
			item: service,
		},
		{
			event: watch.Modified,
			item: service,
		},
		{
			event: watch.Deleted,
			item: service,
		},
	}

	testWatch(tests, ccapi.Services, t)
}

func TestWatchReplicationControllers(t *testing.T) {

	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{Name: "replicationcontroller1", Namespace: "test", ResourceVersion: "18"},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
		},
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: rc,
		},
		{
			event: watch.Added,
			item: rc,
		},
		{
			event: watch.Modified,
			item: rc,
		},
		{
			event: watch.Deleted,
			item: rc,
		},
	}

	testWatch(tests, ccapi.ReplicationControllers, t)
}

func TestWatchPersistentVolumes(t *testing.T) {
	pv := &api.PersistentVolume{
		ObjectMeta: api.ObjectMeta{Name: "persistentvolume1", Namespace: "test", ResourceVersion: "18"},
		Spec: api.PersistentVolumeSpec{
				Capacity: api.ResourceList{
					api.ResourceName(api.ResourceStorage): resource.MustParse("10G"),
				},
				PersistentVolumeSource: api.PersistentVolumeSource{
					HostPath: &api.HostPathVolumeSource{Path: "/foo"},
				},
				PersistentVolumeReclaimPolicy: "Retain",
			},
			Status: api.PersistentVolumeStatus{
				Phase: api.PersistentVolumePhase("Pending"),
			},
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: pv,
		},
		{
			event: watch.Added,
			item: pv,
		},
		{
			event: watch.Modified,
			item: pv,
		},
		{
			event: watch.Deleted,
			item: pv,
		},
	}

	testWatch(tests, ccapi.PersistentVolumes, t)
}

func TestWatchPersistentVolumeClaims(t *testing.T) {
	pvc := &api.PersistentVolumeClaim{
		ObjectMeta: api.ObjectMeta{Name: "persistentVolumeClaim1", Namespace: "test", ResourceVersion: "123"},
		Spec: api.PersistentVolumeClaimSpec{
			VolumeName: "volume",
		},
		Status: api.PersistentVolumeClaimStatus{
			Phase: api.PersistentVolumeClaimPhase("Pending"),
		},
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: pvc,
		},
		{
			event: watch.Added,
			item: pvc,
		},
		{
			event: watch.Modified,
			item: pvc,
		},
		{
			event: watch.Deleted,
			item: pvc,
		},
	}

	testWatch(tests, ccapi.PersistentVolumeClaims, t)
}

func TestWatchNodes(t *testing.T) {
	node := &api.Node{
		ObjectMeta: api.ObjectMeta{Name: "node1", Namespace: "test", ResourceVersion: "123"},
		Spec: api.NodeSpec{
			ExternalID: "ext",
		},
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: node,
		},
		{
			event: watch.Added,
			item: node,
		},
		{
			event: watch.Modified,
			item: node,
		},
		{
			event: watch.Deleted,
			item: node,
		},
	}

	testWatch(tests, ccapi.Nodes, t)
}
