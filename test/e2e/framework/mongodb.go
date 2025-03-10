package framework

import (
	"fmt"
	"strconv"
	"time"

	"github.com/appscode/go/crypto/rand"
	jsonTypes "github.com/appscode/go/encoding/json/types"
	"github.com/appscode/go/types"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
)

var (
	JobPvcStorageSize = "2Gi"
	DBPvcStorageSize  = "1Gi"
)

const (
	kindEviction = "Eviction"
)

func (i *Invocation) MongoDBStandalone() *api.MongoDB {
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mongodb"),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: jsonTypes.StrYo(DBCatalogName),
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
		},
	}
}

func (i *Invocation) MongoDBRS() *api.MongoDB {
	dbName := rand.WithUniqSuffix("mongo-rs")
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version:  jsonTypes.StrYo(DBCatalogName),
			Replicas: types.Int32P(2),
			ReplicaSet: &api.MongoDBReplicaSet{
				Name: dbName,
			},
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
		},
	}
}

func (i *Invocation) MongoDBShard() *api.MongoDB {
	dbName := rand.WithUniqSuffix("mongo-sh")
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: jsonTypes.StrYo(DBCatalogName),
			ShardTopology: &api.MongoDBShardingTopology{
				Shard: api.MongoDBShardNode{
					Shards: 2,
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(i.StorageClass),
					},
				},
				ConfigServer: api.MongoDBConfigNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(i.StorageClass),
					},
				},
				Mongos: api.MongoDBMongosNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
				},
			},
		},
	}
}

func IsRepSet(db *api.MongoDB) bool {
	return db.Spec.ReplicaSet != nil
}

// ClusterAuthModeP returns a pointer to the int32 value passed in.
func ClusterAuthModeP(v api.ClusterAuthMode) *api.ClusterAuthMode {
	return &v
}

// SSLModeP returns a pointer to the int32 value passed in.
func SSLModeP(v api.SSLMode) *api.SSLMode {
	return &v
}

func (i *Invocation) CreateMongoDB(obj *api.MongoDB) error {
	_, err := i.dbClient.KubedbV1alpha1().MongoDBs(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetMongoDB(meta metav1.ObjectMeta) (*api.MongoDB, error) {
	return f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchMongoDB(meta metav1.ObjectMeta, transform func(*api.MongoDB) *api.MongoDB) (*api.MongoDB, error) {
	mongodb, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mongodb, _, err = util.PatchMongoDB(f.dbClient.KubedbV1alpha1(), mongodb, transform)
	return mongodb, err
}

func (f *Framework) DeleteMongoDB(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) EvictPodsFromStatefulSet(meta metav1.ObjectMeta) error {
	var err error
	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: api.GenericKey,
		api.LabelDatabaseKind:       api.ResourceKindMongoDB,
		api.LabelDatabaseName:       meta.GetName(),
	}
	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable)
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}

func (f *Framework) EvictPodsFromDeployment(meta metav1.ObjectMeta) error {
	var err error
	deployName := meta.Name + "-mongos"
	//if PDB is not found, send error
	pdb, err := f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).Get(deployName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pdb.Spec.MinAvailable == nil {
		return fmt.Errorf("found pdb %s spec.minAvailable nil", pdb.Name)
	}

	podSelector := labels.Set{
		api.MongoDBMongosLabelKey: meta.Name + "-mongos",
	}
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(metav1.ListOptions{LabelSelector: podSelector.String()})
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policy.SchemeGroupVersion.String(),
			Kind:       kindEviction,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: meta.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	// try to evict as many pods as allowed in pdb
	minAvailable := pdb.Spec.MinAvailable.IntValue()
	podCount := len(pods.Items)
	for i, pod := range pods.Items {
		eviction.Name = pod.Name
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
		if i < (podCount - minAvailable) {
			if err != nil {
				return err
			}
		} else {
			// This pod should not get evicted
			if kerr.IsTooManyRequests(err) {
				err = nil
				break
			} else if err != nil {
				return err
			} else {
				return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", meta.Namespace, eviction.Name, pdb.Name)
			}
		}
	}
	return err
}

func (f *Framework) EventuallyMongoDB(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			mongodb, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return mongodb.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) CleanMongoDB() {
	mongodbList, err := f.dbClient.KubedbV1alpha1().MongoDBs(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range mongodbList.Items {
		if _, _, err := util.PatchMongoDB(f.dbClient.KubedbV1alpha1(), &e, func(in *api.MongoDB) *api.MongoDB {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}); err != nil {
			fmt.Printf("error Patching MongoDB. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().MongoDBs(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of MongoDB. Error: %v", err)
	}
}
