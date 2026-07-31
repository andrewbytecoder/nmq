package metrics

// ResourceSnapshot describes the currently active topology known to the
// application.
//
// UpdateResources compares the latest snapshot with the previous one and marks
// metrics tied to removed listeners, routes, services, or backend URLs for
// deletion after the next Prometheus scrape.
type ResourceSnapshot struct {
	// Listeners contains the active inbound listener names, such as entrypoint
	// names or logical server ports.
	Listeners []string
	// Routes contains the active route identifiers. For Gin integrations this is
	// typically the registered route pattern, such as "/users/:id".
	Routes []string
	// Services contains the active logical backend service names.
	Services []string
	// BackendURLs contains the active backend target URLs keyed by service name.
	// A service may exist with an empty backend URL list when the service itself
	// is still valid but currently has no concrete upstream target.
	BackendURLs map[string][]string
}

// resourceState is an optimized lookup structure used by cleanup code.
// The public ResourceSnapshot keeps the API ergonomic while this type keeps the
// repeated existence checks cheap during scrapes.
type resourceState struct {
	listeners map[string]bool
	routes    map[string]bool
	services  map[string]map[string]bool
}

// newResourceState returns an empty in-memory topology snapshot.
func newResourceState() *resourceState {
	return &resourceState{
		listeners: make(map[string]bool),
		routes:    make(map[string]bool),
		services:  make(map[string]map[string]bool),
	}
}

// newResourceStateFromSnapshot normalizes the public snapshot into the lookup
// structure used by the collector state.
func newResourceStateFromSnapshot(snapshot ResourceSnapshot) *resourceState {
	state := newResourceState()

	for _, listener := range snapshot.Listeners {
		if listener == "" {
			continue
		}
		state.listeners[listener] = true
	}

	for _, route := range snapshot.Routes {
		if route == "" {
			continue
		}
		state.routes[route] = true
	}

	for _, service := range snapshot.Services {
		if service == "" {
			continue
		}
		state.services[service] = make(map[string]bool)
	}

	for service, urls := range snapshot.BackendURLs {
		if service == "" {
			continue
		}

		if _, ok := state.services[service]; !ok {
			state.services[service] = make(map[string]bool)
		}

		for _, url := range urls {
			if url == "" {
				continue
			}
			state.services[service][url] = true
		}
	}

	return state
}

// snapshot converts the optimized state back into the public snapshot shape.
func (r *resourceState) snapshot() ResourceSnapshot {
	snapshot := ResourceSnapshot{
		Listeners:   make([]string, 0, len(r.listeners)),
		Routes:      make([]string, 0, len(r.routes)),
		Services:    make([]string, 0, len(r.services)),
		BackendURLs: make(map[string][]string, len(r.services)),
	}

	for listener := range r.listeners {
		snapshot.Listeners = append(snapshot.Listeners, listener)
	}

	for route := range r.routes {
		snapshot.Routes = append(snapshot.Routes, route)
	}

	for service, urls := range r.services {
		snapshot.Services = append(snapshot.Services, service)
		snapshot.BackendURLs[service] = make([]string, 0, len(urls))
		for url := range urls {
			snapshot.BackendURLs[service] = append(snapshot.BackendURLs[service], url)
		}
	}

	return snapshot
}

// hasListener reports whether the listener still exists in the active topology.
func (r *resourceState) hasListener(listener string) bool {
	_, ok := r.listeners[listener]
	return ok
}

// hasRoute reports whether the route still exists in the active topology.
func (r *resourceState) hasRoute(route string) bool {
	_, ok := r.routes[route]
	return ok
}

// hasService reports whether the service still exists in the active topology.
func (r *resourceState) hasService(service string) bool {
	_, ok := r.services[service]
	return ok
}

// hasBackendURL reports whether the backend target still belongs to the service.
func (r *resourceState) hasBackendURL(service, url string) bool {
	urls, ok := r.services[service]
	if !ok {
		return false
	}

	_, ok = urls[url]
	return ok
}
