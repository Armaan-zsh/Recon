package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"news-cli/internal/database"
	"news-cli/internal/models"
	"news-cli/internal/scorer"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	db  *database.IntelligenceDB
	mux *http.ServeMux
}

func NewServer(db *database.IntelligenceDB) *Server {
	s := &Server{db: db, mux: http.NewServeMux()}

	s.mux.HandleFunc("/api/latest", s.handleLatest)
	s.mux.HandleFunc("/api/latest/lanes", s.handleLatestLanes)
	s.mux.HandleFunc("/api/headline", s.handleHeadline)
	s.mux.HandleFunc("/api/archive/", s.handleArchive)
	s.mux.HandleFunc("/api/entities/graph", s.handleEntityGraph)
	s.mux.HandleFunc("/api/entities/trending", s.handleTrending)

	return s
}

type laneArticle struct {
	models.Article
	GeneralScore int      `json:"general_score"`
	ExpertScore  int      `json:"expert_score"`
	Why          []string `json:"why"`
}

type lanePayload struct {
	GeneratedAt string        `json:"generated_at"`
	General     []laneArticle `json:"general"`
	Expert      []laneArticle `json:"expert"`
}

func (s *Server) Listen(ctx context.Context, port int) error {
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		cancel()
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	s.writeJSON(w, map[string]string{"error": msg})
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	arts, err := s.db.GetRecentArticles(20)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	s.writeJSON(w, arts)
}

func (s *Server) handleLatestLanes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	arts, err := s.db.GetRecentArticles(60)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	general := make([]laneArticle, 0, len(arts))
	expert := make([]laneArticle, 0, len(arts))
	for _, art := range arts {
		signals := scorer.ScoreLanes(art)
		general = append(general, laneArticle{
			Article:      art,
			GeneralScore: signals.GeneralScore,
			ExpertScore:  signals.ExpertScore,
			Why:          signals.GeneralWhy,
		})
		expert = append(expert, laneArticle{
			Article:      art,
			GeneralScore: signals.GeneralScore,
			ExpertScore:  signals.ExpertScore,
			Why:          signals.ExpertWhy,
		})
	}

	sort.Slice(general, func(i, j int) bool {
		if general[i].GeneralScore == general[j].GeneralScore {
			return general[i].Published.After(general[j].Published)
		}
		return general[i].GeneralScore > general[j].GeneralScore
	})
	sort.Slice(expert, func(i, j int) bool {
		if expert[i].ExpertScore == expert[j].ExpertScore {
			return expert[i].Published.After(expert[j].Published)
		}
		return expert[i].ExpertScore > expert[j].ExpertScore
	})

	if len(general) > 20 {
		general = general[:20]
	}
	if len(expert) > 20 {
		expert = expert[:20]
	}

	s.writeJSON(w, lanePayload{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		General:     general,
		Expert:      expert,
	})
}

func (s *Server) handleHeadline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	arts, err := s.db.GetRecentArticles(1)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	if len(arts) == 0 {
		s.writeError(w, http.StatusNotFound, "no articles")
		return
	}
	s.writeJSON(w, arts[0])
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	date := strings.TrimPrefix(r.URL.Path, "/api/archive/")
	date = strings.TrimSpace(date)
	if !dateRe.MatchString(date) {
		s.writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	arts, err := s.db.GetArticlesByDate(date, 2000)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	s.writeJSON(w, arts)
}

func (s *Server) handleTrending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ents, err := s.db.GetTrendingEntities(24, 50)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	s.writeJSON(w, ents)
}

func (s *Server) handleEntityGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	nodes, edges, err := s.db.GetEntityGraph()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	s.writeJSON(w, map[string]any{"nodes": nodes, "edges": edges})
}

func (s *Server) DebugString(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
