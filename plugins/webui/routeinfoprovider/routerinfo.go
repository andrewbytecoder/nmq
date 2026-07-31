package routeinfoprovider

import (
	"fmt"

	"ysp.com/ncp/ncp/interfaces/dpproxy"
)

type RouteInfoProvider struct {
	r dpproxy.IRouter
}

func New(r dpproxy.IRouter) *RouteInfoProvider {
	return &RouteInfoProvider{
		r: r,
	}
}

type RoutesInfo []dpproxy.RouteInfo

type IRouteInfoProvider interface {
	ListRouters() (RoutesInfo, error)
}

func (p *RouteInfoProvider) ListRouters() (routes RoutesInfo, err error) {
	if p == nil || p.r == nil {
		return nil, fmt.Errorf("route provider is unavailable")
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			routes = nil
			err = fmt.Errorf("list routers panic: %v", recovered)
		}
	}()

	routes = p.r.GetRoutes()
	return routes, nil
}
