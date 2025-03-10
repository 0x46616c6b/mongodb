package framework

import (
	"errors"
	"time"

	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) EventuallyCRD() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			// Check MongoDB TPR
			if _, err := f.dbClient.KubedbV1alpha1().MongoDBs(core.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD MongoDB is not ready")
			}

			// Check Snapshots TPR
			if _, err := f.dbClient.KubedbV1alpha1().Snapshots(core.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD Snapshot is not ready")
			}

			// Check DormantDatabases TPR
			if _, err := f.dbClient.KubedbV1alpha1().DormantDatabases(core.NamespaceAll).List(metav1.ListOptions{}); err != nil {
				return errors.New("CRD DormantDatabase is not ready")
			}

			return nil
		},
		time.Minute*2,
		time.Second*10,
	)
}
