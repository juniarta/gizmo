package service

import (
	"net/http"
	"strconv"

	"github.com/NYTimes/gizmo/config"
	"github.com/NYTimes/gizmo/server"
	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/context"
)

// SavedItemsService will keep a handle on the saved items repository and implement
// the gizmo/server.JSONService interface.
type SavedItemsService struct {
	repo SavedItemsRepo
}

// NewSavedItemsService will attempt to instantiate a new repository and service.
func NewSavedItemsService(cfg *config.MySQL) (*SavedItemsService, error) {
	repo, err := NewSavedItemsRepo(cfg)
	if err != nil {
		return nil, err
	}
	return &SavedItemsService{repo}, nil
}

// Prefix is to implement gizmo/server.Service interface. The string will be prefixed to all endpoint
// routes.
func (s *SavedItemsService) Prefix() string {
	return "/svc/saved-items"
}

type idKey int

const userIDKey idKey = 0

// Middleware provides a hook to add service-wide middleware to the service. In this example
// we are using it to add GZIP compression to our responses.
func (s *SavedItemsService) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// wrap the response with our GZIP Middleware
		h = gziphandler.GzipHandler(h)
		// and call the endpoint
		h.ServeHTTP(w, r)
	})
}

// JSONMiddleware provides a hook to add service-wide middleware for how JSONEndpoints
// should behave. In this example, we’re catching all errors and providing a generic JSON
// response.
func (s *SavedItemsService) JSONMiddleware(j server.JSONEndpoint) server.JSONEndpoint {
	return func(r *http.Request) (code int, res interface{}, err error) {

		// wrap our endpoint with an auth check
		code, res, err = authCheck(j)(r)

		// if the endpoint returns an unexpected error, return a generic message
		// and log it.
		if err != nil && code != http.StatusUnauthorized {
			server.Log.WithField("error", err).Error("unexpected service error")
			return http.StatusServiceUnavailable, nil, ServiceUnavailableErr
		}

		return code, res, err
	}
}

func authCheck(j server.JSONEndpoint) server.JSONEndpoint {
	return func(r *http.Request) (code int, res interface{}, err error) {
		// check for User ID header injected by API Gateway
		idStr := r.Header.Get("USER_ID")
		// verify it's an int
		id, err := strconv.ParseUint(idStr, 10, 64)
		// reject request if bad/no user ID
		if err != nil || id == 0 {
			return http.StatusUnauthorized, nil, UnauthErr
		}
		// set the ID in context if we're good
		context.Set(r, userIDKey, id)

		return j(r)
	}
}

// JSONEndpoints is the most important method of the Service implementation. It provides a
// listing of all endpoints available in the service with their routes and HTTP methods.
func (s *SavedItemsService) JSONEndpoints() map[string]map[string]server.JSONEndpoint {
	return map[string]map[string]server.JSONEndpoint{
		"/user": map[string]server.JSONEndpoint{
			"GET":    s.Get,
			"PUT":    s.Put,
			"DELETE": s.Delete,
		},
	}
}

type (
	// jsonResponse is a generic struct for responding with a simple JSON message.
	jsonResponse struct {
		Message string `json:"message"`
	}
	// jsonErr is a tiny helper struct to make displaying errors in JSON better.
	jsonErr struct {
		Err string `json:"error"`
	}
)

func (e *jsonErr) Error() string {
	return e.Err
}

var (
	// ServiceUnavailableErr is a global error that will get returned when we are experiencing
	// technical issues.
	ServiceUnavailableErr = &jsonErr{"sorry, this service is currently unavailable"}
	// UnauthErr is a global error returned when the user does not supply the proper
	// authorization headers.
	UnauthErr = &jsonErr{"please include a valid USER_ID header in the request"}
)
