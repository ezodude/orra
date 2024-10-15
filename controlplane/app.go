package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gilcrest/diygoapi/errs"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const JSONMarshalingFail = "Orra:JSONMarshalingFail"

type App struct {
	Plane  *ControlPlane
	Router *mux.Router
	Cfg    Config
	Logger zerolog.Logger
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func NewApp(cfg Config, args []string) (*App, error) {
	lgr, err := NewLogger(args)
	if err != nil {
		return nil, err
	}

	return &App{
		Logger: lgr,
		Cfg:    cfg,
	}, nil
}

func (app *App) configureRoutes() *App {
	app.Router.HandleFunc("/register/project", app.RegisterProject).Methods("POST")
	app.Router.HandleFunc("/register/service", app.APIKeyMiddleware(app.RegisterService)).Methods("POST")
	app.Router.HandleFunc("/orchestrations", app.APIKeyMiddleware(app.OrchestrationsHandler)).Methods("POST")
	app.Router.HandleFunc("/register/agent", app.APIKeyMiddleware(app.RegisterAgent)).Methods("POST")
	app.Router.HandleFunc("/ws", app.HandleWebSocket)
	return app
}

func (app *App) Run() {
	port := app.Cfg.Port
	addr := fmt.Sprintf(":%d", port)

	srv := &http.Server{
		Addr: addr,
		// Good practice to set timeouts to avoid Slowloris attacks.
		// See: https://en.wikipediapp.org/wiki/Slowloris_(computer_security)
		WriteTimeout: time.Second * 180,
		ReadTimeout:  time.Second * 180,
		IdleTimeout:  time.Second * 180,
		Handler:      app.Router,
	}

	// Set up our server in s goroutine so that it doesn't block.
	go func() {
		app.Logger.Info().Msgf("Starting control plane on %s", addr)
		if err := srv.ListenAndServe(); err != nil {
			app.Logger.Info().Msg(err.Error())
		}
	}()

	c := make(chan os.Signal, 1)

	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create s deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	_ = srv.Shutdown(ctx)

	app.Logger.Debug().Msg("http: All connections drained")
}

func (app *App) RegisterProject(w http.ResponseWriter, r *http.Request) {
	var project Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.InvalidRequest, errs.Code(JSONMarshalingFail), err))
		return
	}

	project.ID = uuid.New().String()
	project.APIKey = uuid.New().String()

	app.Plane.projects[project.ID] = &project

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unanticipated, errs.Code(JSONMarshalingFail), err))
		return
	}
}

func (app *App) RegisterServiceOrAgent(w http.ResponseWriter, r *http.Request, serviceType ServiceType) {
	apiKey := r.Context().Value("api_key").(string)
	project, err := app.Plane.GetProjectByApiKey(apiKey)
	if err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.InvalidRequest, err))
		return
	}

	var service ServiceInfo
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.InvalidRequest, errs.Code(JSONMarshalingFail), err))
		return
	}

	service.ID = uuid.New().String()
	service.ProjectID = project.ID
	service.Type = serviceType

	// need a better to add services that avoid duplicating service registration
	app.Plane.services[project.ID] = append(app.Plane.services[project.ID], &service)
	app.Plane.wsConnectionsMutex.Lock()
	app.Plane.wsConnections[service.ID] = &ServiceConnection{
		Status: Disconnected,
	}
	app.Plane.wsConnectionsMutex.Unlock()

	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":     service.ID,
		"name":   service.Name,
		"status": Registered,
	}); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unanticipated, err))
		return
	}
}

func (app *App) RegisterService(w http.ResponseWriter, r *http.Request) {
	app.RegisterServiceOrAgent(w, r, Service)
}

func (app *App) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	app.RegisterServiceOrAgent(w, r, Agent)
}

func (app *App) OrchestrationsHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Context().Value("api_key").(string)
	project, err := app.Plane.GetProjectByApiKey(apiKey)
	if err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unanticipated, err))
		return
	}

	var orchestration Orchestration
	if err := json.NewDecoder(r.Body).Decode(&orchestration); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.InvalidRequest, errs.Code(JSONMarshalingFail), err))
		return
	}

	orchestration.ID = uuid.New().String()
	orchestration.Status = Pending
	orchestration.ProjectID = project.ID

	app.Plane.prepareOrchestration(&orchestration)

	if !orchestration.Executable() {
		app.Logger.
			Debug().
			Str("Status", orchestration.Status.String()).
			Msgf("Orchestration %s cannot be executed: %s", orchestration.ID, orchestration.Error)
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else {
		app.Logger.Debug().Msgf("About to execute orchestration %s", orchestration.ID)
		go app.Plane.executeOrchestration(&orchestration)
		w.WriteHeader(http.StatusAccepted)
	}

	data, err := json.Marshal(orchestration)
	if err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unanticipated, errs.Code(JSONMarshalingFail), err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(data); err != nil {
		errs.HTTPErrorResponse(w, app.Logger, errs.E(errs.Unanticipated, err))
		return
	}
}

func (app *App) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	serviceId := r.URL.Query().Get("serviceId")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		app.Logger.Error().Msgf("Failed to upgrade the HTTP server connection to the WebSocket protocol for service %s.", serviceId)
		return
	}

	app.Plane.wsConnectionsMutex.Lock()
	serviceConn, ok := app.Plane.wsConnections[serviceId]
	if !ok {
		app.Logger.Debug().Msgf("ServiceID %s not registered", serviceId)
		if err := conn.Close(); err != nil {
			app.Logger.Error().Msgf("Error closing websocket after discovering ServiceID %s not registered", serviceId)
			return
		}
		return
	}
	serviceConn.Conn = conn
	serviceConn.Status = Connected

	app.Plane.wsConnectionsMutex.Unlock()
}