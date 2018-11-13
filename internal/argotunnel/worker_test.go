package argotunnel

import (
	"fmt"
	"testing"

	"k8s.io/client-go/util/workqueue"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestSync(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		w   worker
		key string
		err error
	}{
		"sync-key-err": {
			w: worker{
				translator: &mockTranslator{},
				queue:      &mockQueue{},
			},
			key: "kind-no-meta",
			err: fmt.Errorf("unexpected key format: %q", "kind-no-meta"),
		},
		"sync-okay": {
			w: worker{
				translator: func() translator {
					t := &mockTranslator{}
					t.On("handleResource", "kind", "namespace/name").Return(nil)
					return t
				}(),
				queue: &mockQueue{},
			},
			key: "kind/namespace/name",
			err: nil,
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.w.log = logger

		err := test.w.sync(test.key)
		assert.Equalf(t, test.err, err, "test '%s' error mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}

func TestProcessNextItem(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		w   worker
		out bool
	}{
		"process-quit": {
			w: worker{
				translator: &mockTranslator{},
				queue: func() workqueue.RateLimitingInterface {
					q := &mockQueue{}
					q.On("Get").Return("", true)
					return q
				}(),
				options: options{},
			},
			out: false,
		},
		"process-sync-error-requeue": {
			w: worker{
				translator: &mockTranslator{},
				queue: func() workqueue.RateLimitingInterface {
					q := &mockQueue{}
					q.On("Get").Return("kind", false)
					q.On("Done", "kind").Return()
					q.On("NumRequeues", "kind").Return(1)
					q.On("AddRateLimited", "kind").Return()
					return q
				}(),
				options: options{
					requeueLimit: 2,
				},
			},
			out: true,
		},
		"process-sync-error-forget": {
			w: worker{
				translator: &mockTranslator{},
				queue: func() workqueue.RateLimitingInterface {
					q := &mockQueue{}
					q.On("Get").Return("kind", false)
					q.On("Done", "kind").Return()
					q.On("NumRequeues", "kind").Return(2)
					q.On("Forget", "kind").Return()
					return q
				}(),
				options: options{
					requeueLimit: 2,
				},
			},
			out: true,
		},
		"process-sync-okay": {
			w: worker{
				translator: func() translator {
					t := &mockTranslator{}
					t.On("handleResource", "kind", "namespace/name").Return(nil)
					return t
				}(),
				queue: func() workqueue.RateLimitingInterface {
					q := &mockQueue{}
					q.On("Get").Return("kind/namespace/name", false)
					q.On("Done", "kind/namespace/name").Return()
					q.On("Forget", "kind/namespace/name").Return()
					return q
				}(),
				options: options{
					requeueLimit: 2,
				},
			},
			out: true,
		},
	} {
		logger, hook := logtest.NewNullLogger()
		test.w.log = logger

		out := test.w.processNextItem()
		assert.Equalf(t, test.out, out, "test '%s' condition mismatch", name)
		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}
}
