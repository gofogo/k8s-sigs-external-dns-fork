package source

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// AddEventHandler adds an event handler that should be triggered if the watched Istio Gateway changes.
func (sc *gatewaySource) AddEventHandler(ctx context.Context, handler func()) {
	log.Debug("Adding event handler for Istio Gateway")
	if sc.gatewayInformerV1 != nil {
		sc.gatewayInformerV1.Informer().AddEventHandler(eventHandlerFunc(handler))
	} else {
		sc.gatewayInformerV1Alpha3.Informer().AddEventHandler(eventHandlerFunc(handler))
	}
}
