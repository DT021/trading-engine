package engine

import "sync"

type portfolioHandler struct {
	trades []*Trade
	mut    *sync.RWMutex
}

func newPortfolio() *portfolioHandler {
	p := portfolioHandler{}
	p.mut = &sync.RWMutex{}
	return &p
}

func (p *portfolioHandler) onNewTrade(t *Trade) {
	p.mut.Lock()
	p.trades = append(p.trades, t)
	p.mut.Unlock()
}

func (p *portfolioHandler) totalPnL() float64 {
	p.mut.RLock()
	defer p.mut.RUnlock()
	pnl := 0.0
	for _, pos := range p.trades {
		if pos.Type == FlatTrade {
			continue
		}
		pnl += pos.ClosedPnL + pos.OpenPnL
	}
	return pnl
}

func (p *portfolioHandler) openPnL() float64 {
	p.mut.RLock()
	defer p.mut.RUnlock()
	pnl := 0.0
	for _, pos := range p.trades {
		if pos.Type == FlatTrade || pos.Type == ClosedTrade {
			continue
		}
		pnl += pos.OpenPnL
	}
	return pnl
}

func (p *portfolioHandler) ClosedPnL() float64 {
	p.mut.RLock()
	defer p.mut.RUnlock()
	pnl := 0.0
	for _, pos := range p.trades {
		if pos.Type == FlatTrade {
			continue
		}
		pnl += pos.ClosedPnL
	}
	return pnl

}
