package main

import (
	"os"
	"time"

	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/bbolt"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/events"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/nodes"
	"github.com/dollarshaveclub/node-auto-repair-operator/pkg/store"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	// GitSHA is set by a build argument.
	GitSHA string
)

// k8sConfig parses a local kubeconfig file. If it doesn't exist, the
// function assumes that it's running in a Kubernetes cluster, so it
// attempts to parse the service account configuration.
func k8sConfig() (*rest.Config, error) {
	// TODO: parse from flag
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err == nil {
		logrus.Debug("using kubeconfig configuration")
		return config, nil
	}

	logrus.WithError(err).Debug("did not find kubeconfig file")

	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "error fetching in-cluster Kubernetes configuration")
	}

	logrus.Debug("using in-cluster Kubernetes configuration")

	return config, nil
}

func k8sClient() (kubernetes.Interface, error) {
	conf, err := k8sConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "error fetching Kubernetes configuration")
	}
	client, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating Kubernetes clientset")
	}

	return client, nil
}

func main() {
	// TODO: make this configurable
	logrus.SetLevel(logrus.DebugLevel)

	viper.SetEnvPrefix("naro")
	viper.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "node-auto-repair-operator",
		Short: "node-auto-repair-operator repairs faulty nodes in a Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
			logrus.Infof("starting node-auto-repair-operator")
			logrus.Infof("build ref: %s", GitSHA)
			logrus.Infof("using database at %s", viper.GetString("db"))

			db, err := bolt.Open(viper.GetString("db"), 0600, nil)
			if err != nil {
				logrus.Fatal(err)
			}
			s, err := store.NewStore(db)
			if err != nil {
				logrus.Fatal(err)
			}

			k8s, err := k8sClient()
			if err != nil {
				logrus.Fatal(err)
			}

			pollInterval := time.Second * 5

			eventInformer := v1informers.NewEventInformer(k8s,
				"default",
				pollInterval,
				cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

			eventController := events.NewKubeNodeEventController(db, k8s.CoreV1().Nodes(), s)

			eventEmitter := events.NewKubeNodeEventEmitter(eventInformer, pollInterval)
			eventEmitter.AddHandler(eventController)
			eventEmitter.Start()

			// Block forever
			var c chan struct{}
			<-c
		},
	}
	rootCmd.PersistentFlags().String("db", "/tmp/node-auto-repair-operator.db", "the path to the embedded database")
	viper.BindPFlag("db", rootCmd.PersistentFlags().Lookup("db"))

	exportDBCmd := &cobra.Command{
		Use:   "export-db",
		Short: "exports the database as a JSON file",
		Run: func(cmd *cobra.Command, args []string) {
			logrus.Infof("using database at %s", viper.GetString("db"))
			logrus.Infof("exporting database to %s", viper.GetString("file"))

			db, err := bolt.Open(viper.GetString("db"), 0600, nil)
			if err != nil {
				logrus.Fatal(err)
			}
			s, err := store.NewStore(db)
			if err != nil {
				logrus.Fatal(err)
			}

			var export struct {
				NodeTimePeriodSummaries []*nodes.NodeTimePeriodSummary
			}

			export.NodeTimePeriodSummaries, err = s.GetNodeTimePeriodSummaries(time.Now().Add(-365*24*time.Hour), time.Now())
			if err != nil {
				logrus.Fatal(err)
			}

			file, err := os.Create(viper.GetString("file"))
			if err != nil {
				logrus.Fatal(err)
			}
			defer file.Close()

			enc := json.NewEncoder(file)
			enc.SetIndent("", "    ")

			if err := enc.Encode(export); err != nil {
				logrus.Fatal(err)
			}
		},
	}
	exportDBCmd.Flags().String("file", "/tmp/node-auto-repair-operator-export.json", "where to export the database to")
	viper.BindPFlag("file", exportDBCmd.Flags().Lookup("file"))
	rootCmd.AddCommand(exportDBCmd)

	rootCmd.Execute()
}
