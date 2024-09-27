package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type App struct {
	op     *ControlPlane
	router *mux.Router
	port   int
}

func NewApp(oPlatform *ControlPlane, router *mux.Router, port int) *App {
	return &App{
		op:     oPlatform,
		router: router,
		port:   port,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for this example
	},
}

func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is missing", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		apiKey := parts[1]

		// Store the API key in the request context
		ctx := context.WithValue(r.Context(), "api_key", apiKey)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}
}

func (app *App) configureRoutes() *App {
	app.router.HandleFunc("/register/project", app.RegisterProject).Methods("POST")
	app.router.HandleFunc("/register/service", APIKeyMiddleware(app.RegisterService)).Methods("POST")
	app.router.HandleFunc("/orchestrations", APIKeyMiddleware(app.OrchestrationsHandler)).Methods("POST")
	app.router.HandleFunc("/register/agent", APIKeyMiddleware(app.RegisterAgent)).Methods("POST")
	app.router.HandleFunc("/ws", app.HandleWebSocket)
	return app
}

func (app *App) run() error {
	log.Printf("Starting server on :%d\n", app.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", app.port), app.router)
}

func (app *App) RegisterProject(w http.ResponseWriter, r *http.Request) {
	var project Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	project.ID = uuid.New().String()
	project.APIKey = uuid.New().String()

	app.op.projects[project.ID] = &project

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) RegisterServiceOrAgent(w http.ResponseWriter, r *http.Request, serviceType ServiceType) {
	apiKey := r.Context().Value("api_key").(string)
	project, err := app.op.GetProjectByApiKey(apiKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var service ServiceInfo
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	service.ID = uuid.New().String()
	service.ProjectID = project.ID
	service.Type = serviceType

	// need a better to add services that avoid duplicating service registration
	app.op.services[project.ID] = append(app.op.services[project.ID], &service)
	app.op.wsConnections[service.ID] = &ServiceConnection{
		Status: Disconnected,
	}

	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":     service.ID,
		"name":   service.Name,
		"status": Registered,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	project, err := app.op.GetProjectByApiKey(apiKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var orchestration Orchestration
	if err := json.NewDecoder(r.Body).Decode(&orchestration); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	orchestration.ID = uuid.New().String()
	orchestration.Status = Pending
	orchestration.ProjectID = project.ID

	app.op.orchestrationStore[orchestration.ID] = &orchestration
	app.op.prepareOrchestration(&orchestration)

	if orchestration.Status == NotActionable || orchestration.Status == Failed {
		w.WriteHeader(http.StatusUnprocessableEntity)
	} else {
		go app.op.executeOrchestration(&orchestration)
		w.WriteHeader(http.StatusAccepted)
	}

	data, err := json.Marshal(orchestration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	serviceId := r.URL.Query().Get("serviceId")
	app.op.mu.Lock()
	serviceConn, ok := app.op.wsConnections[serviceId]
	if !ok {
		log.Printf("ServiceID %s not registered", serviceId)
		if err := conn.Close(); err != nil {
			log.Printf("Error closing connection as after discovering ServiceID %s not registered\n", serviceId)
			return
		}
		return
	}
	serviceConn.Conn = conn
	serviceConn.Status = Connected

	app.op.mu.Unlock()
}

func main() {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}

	op := NewControlPlane(cfg.OpenApiKey)
	r := mux.NewRouter()
	app := NewApp(op, r, cfg.Port).configureRoutes()
	log.Fatal(app.run())
}
