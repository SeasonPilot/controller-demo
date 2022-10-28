package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SeasonPilot/controller-demo/pkg/client/clientset/versioned"
	informers "github.com/SeasonPilot/controller-demo/pkg/client/informers/externalversions"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

var (
	onlyOneSignalHandler = make(chan struct{})
)

func main() {
	// 1. clientset
	_, cronTabClient, err := initClinet()
	if err != nil {
		klog.Fatal(err)
	}

	// 2. informer
	sharedInformerFactory := informers.NewSharedInformerFactory(cronTabClient, time.Second*30)
	cronTabInformer := sharedInformerFactory.Stable().V1beta1().CronTabs()

	stopCh := gracefullyStop()

	// 入参 informer 类型
	controller := NewController(cronTabInformer)

	// 4. 启动 informer，开始List & Watch。  一定要在 informer.Informer() 后面
	go sharedInformerFactory.Start(stopCh)

	// 6. 自定义控制器 crontab controller
	controller.run(1, stopCh)
}

func initClinet() (*kubernetes.Clientset, *versioned.Clientset, error) {
	var (
		kubeconfig *string
		config     *rest.Config
		err        error
	)

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "")
	}
	flag.Parse()

	if config, err = rest.InClusterConfig(); err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return nil, nil, err
		}
	}

	k8client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	cronTabclient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return k8client, cronTabclient, err
}

func gracefullyStop() chan struct{} {
	// 当调用两次的时候 panics
	close(onlyOneSignalHandler)

	stopCh := make(chan struct{})

	// 3. 优雅退出
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		close(stopCh)
		<-quit
		os.Exit(1)
	}()

	return stopCh
}
