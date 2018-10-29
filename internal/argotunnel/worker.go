package argotunnel

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type worker struct {
	queue      workqueue.RateLimitingInterface
	translator translator
	log        *logrus.Logger
	options    options
}

func (w *worker) run(stopCh <-chan struct{}) error {
	w.log.Debugf("starting argo-tunnel workers...")
	w.translator.run(stopCh)

	w.log.Infof("synchronizing argo-tunnel caches...")
	if !w.translator.waitForCacheSync(stopCh) {
		return fmt.Errorf("timed out waiting for informer caches to sync")
	}
	w.log.Debugf("spawning argo-tunnel workers...")
	// TODO: convert to semaphore pattern
	for i := 0; i < w.options.workers; i++ {
		go wait.Until(w.work, 1*time.Second, stopCh)
	}
	<-stopCh
	w.log.Debugf("stopping argo-tunnel workers...")
	return nil
}

func (w *worker) work() {
	for w.processNextItem() {
	}
}

func (w *worker) processNextItem() bool {
	key, quit := w.queue.Get()
	if quit {
		return false
	}
	defer w.queue.Done(key)

	if err := w.sync(key.(string)); err == nil {
		w.queue.Forget(key)
	} else if w.queue.NumRequeues(key) < 2 {
		w.queue.AddRateLimited(key)
	} else {
		w.queue.Forget(key)
	}
	return true
}

func (w *worker) sync(key string) error {
	kind, metakey, err := splitKindMetaKey(key)
	if err != nil {
		return err
	}
	return w.translator.handleResource(kind, metakey)
}
