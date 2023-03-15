//go:build darwin

package nexodus

import (
	"context"
	"sync"
)

func (ax *Nexodus) CtlServerStart(ctx context.Context, wg *sync.WaitGroup) error {
	ax.logger.Debugf("Ctl interface not yet supported on OSX")
	return nil
}