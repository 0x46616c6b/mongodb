package e2e_test

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/appscode/go/homedir"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	kext_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	ka "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"kmodules.xyz/client-go/logs"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	"kubedb.dev/apimachinery/client/clientset/versioned/scheme"
	"kubedb.dev/mongodb/test/e2e/framework"
	scs "stash.appscode.dev/stash/client/clientset/versioned"
)

// To Run E2E tests:
// - For selfhosted Operator: ./hack/make.py test e2e --selfhosted-operator=true (--storageclass=standard) (--ginkgo.flakeAttempts=2)
// - For non selfhosted Operator: ./hack/make.py test e2e (--docker-registry=kubedb) (--storageclass=standard) (--ginkgo.flakeAttempts=2)
// () => Optional

var (
	storageClass = "standard"
)

func init() {
	scheme.AddToScheme(clientSetScheme.Scheme)

	flag.StringVar(&storageClass, "storageclass", storageClass, "Kubernetes StorageClass name")
	flag.StringVar(&framework.DockerRegistry, "docker-registry", framework.DockerRegistry, "User provided docker repository")
	flag.StringVar(&framework.DBCatalogName, "db-catalog", framework.DBCatalogName, "MongoDB version")
	flag.BoolVar(&framework.SelfHostedOperator, "selfhosted-operator", framework.SelfHostedOperator, "Enable this for provided controller")
}

const (
	TIMEOUT = 20 * time.Minute
)

var (
	root *framework.Framework
)

func TestE2e(t *testing.T) {
	logs.InitLogs()
	defer logs.FlushLogs()
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(TIMEOUT)

	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "e2e Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {

	userHome := homedir.HomeDir()

	// Kubernetes config
	kubeconfigPath := filepath.Join(userHome, ".kube/config")
	By("Using kubeconfig from " + kubeconfigPath)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	Expect(err).NotTo(HaveOccurred())
	// raise throttling time. ref: https://github.com/appscode/voyager/issues/640
	config.Burst = 100
	config.QPS = 100

	// Clients
	kubeClient := kubernetes.NewForConfigOrDie(config)
	dbClient := cs.NewForConfigOrDie(config)
	kaClient := ka.NewForConfigOrDie(config)
	appCatalogClient := appcat_cs.NewForConfigOrDie(config)
	aPIExtKubeClient := kext_cs.NewForConfigOrDie(config)
	stashClient := scs.NewForConfigOrDie(config)

	// Framework
	root = framework.New(config, kubeClient, aPIExtKubeClient, dbClient, kaClient, appCatalogClient, stashClient, storageClass)

	// Create namespace
	By("Using namespace " + root.Namespace())
	err = root.CreateNamespace()
	Expect(err).NotTo(HaveOccurred())

	if !framework.SelfHostedOperator {
		stopCh := genericapiserver.SetupSignalHandler()
		go root.RunOperatorAndServer(config, kubeconfigPath, stopCh)
	}

	root.EventuallyCRD().Should(Succeed())
	root.EventuallyAPIServiceReady().Should(Succeed())
})

var _ = AfterSuite(func() {

	By("Cleanup Left Overs")
	if !framework.SelfHostedOperator {
		By("Delete Admission Controller Configs")
		root.CleanAdmissionConfigs()
	}
	By("Delete left over MongoDB objects")
	root.CleanMongoDB()
	By("Delete left over Dormant Database objects")
	root.CleanDormantDatabase()
	By("Delete left over Snapshot objects")
	root.CleanSnapshot()
	By("Delete left over workloads if exists any")
	root.CleanWorkloadLeftOvers()
	By("Delete Namespace")
	err := root.DeleteNamespace()
	Expect(err).NotTo(HaveOccurred())
})
