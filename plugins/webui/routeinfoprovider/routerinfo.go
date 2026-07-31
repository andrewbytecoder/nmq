package routeinfoprovider

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

type RouteInfoProvider struct {
	r gin.IRouter
}

func New(r gin.IRouter) *RouteInfoProvider {
	return &RouteInfoProvider{
		r: r,
	}
}

type RoutesInfo []gin.RouteInfo

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

	//routes = p.r.GetRoutes()
	//return routes, nil
	return nil, nil
}
