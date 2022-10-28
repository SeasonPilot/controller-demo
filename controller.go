package main

import (
	"fmt"
	"time"

	"github.com/SeasonPilot/controller-demo/pkg/apis/stable/v1beta1"
	beta1 "github.com/SeasonPilot/controller-demo/pkg/client/informers/externalversions/stable/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

// 7. 定义 controller
type Controller struct {
	workqueue workqueue.RateLimitingInterface
	informer  beta1.CronTabInformer
}

func NewController(informer beta1.CronTabInformer) *Controller {
	c := &Controller{
		workqueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		informer:  informer,
	}

	klog.Info("Setting up crontab event handlers")

	// 5. 给 indexInformer 注册 监听处理函数
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAddFunc,
		UpdateFunc: c.onUpdateFunc,
		DeleteFunc: c.onDelFunc,
	})

	return c
}

func (c *Controller) run(thread int, stopCh chan struct{}) {
	// fixme:
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// fixme: 等待对应资源 crontab 完成同步
	if !cache.WaitForCacheSync(stopCh, c.informer.Informer().HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}
	klog.Infof("informer cache to sync completed")

	for i := 0; i < thread; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shut := c.workqueue.Get() // 对象键
	if shut {
		return false
	}

	// 主要做 key 的类型断言
	err := func(obj interface{}) error {
		var (
			key string
			ok  bool
		)

		// fixme:
		defer c.workqueue.Done(obj)

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj) // fixme:这里入参是 对象键
			runtime.HandleError(fmt.Errorf("expired string but get %v", obj))
			return nil
		}
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	runtime.HandleError(err)

	return true
}

func (c *Controller) syncHandler(key string) error {
	// fixme:
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}

	// 从 indexer 本地存储 中取出对象
	// fixme: 这里不用 indexer,而是用 lister 去拿
	//   obj, exist, err := c.indexer.GetByKey(key)
	cronTab, err := c.informer.Lister().CronTabs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// 对应的 crontab 对象已经被删除了。   对象键存在，对象不存在
			klog.Warningf("[CronTabCRD] %s/%s does not exist in local cache, will delete it from CronTab ...",
				namespace, name)
			klog.Infof("[CronTabCRD] deleting crontab: %s/%s ...", namespace, name)
			return nil
		}
		runtime.HandleError(fmt.Errorf("failed to get crontab by: %s/%s", namespace, name))
		return err
	}

	klog.Infof("[CronTabCRD] try to process crontab: %#v ...", cronTab.Name)

	return nil
}

// indexInformer 监听处理函数
func (c *Controller) onAddFunc(obj interface{}) { // 传入的 obj 类型是 *v1beta1.CronTab
	// fixme: 这里不是断言。
	//  if o, ok := obj.(*v1beat1.CronTab); !ok {

	key, err := cache.MetaNamespaceKeyFunc(obj) // 通过对象获取 对象键
	if err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) onUpdateFunc(oldObj, newObj interface{}) {
	old := oldObj.(*v1beta1.CronTab)
	new := newObj.(*v1beta1.CronTab)

	if old.ResourceVersion == new.ResourceVersion {
		return
	}
	c.onAddFunc(new)
}

func (c *Controller) onDelFunc(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.workqueue.AddRateLimited(key)
}
