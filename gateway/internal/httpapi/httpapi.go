package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	authpb "github.com/sonni-a/minibank/api/auth"
	paymentpb "github.com/sonni-a/minibank/api/payment"
	userpb "github.com/sonni-a/minibank/api/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Server struct {
	authConn  *grpc.ClientConn
	userConn  *grpc.ClientConn
	payConn   *grpc.ClientConn
	auth      authpb.AuthServiceClient
	user      userpb.UserServiceClient
	payment   paymentpb.PaymentServiceClient
	mux       *http.ServeMux
}

func New(authAddr, userAddr, paymentAddr string) (*Server, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	authConn, err := grpc.NewClient(authAddr, opts...)
	if err != nil {
		return nil, err
	}
	userConn, err := grpc.NewClient(userAddr, opts...)
	if err != nil {
		_ = authConn.Close()
		return nil, err
	}
	payConn, err := grpc.NewClient(paymentAddr, opts...)
	if err != nil {
		_ = authConn.Close()
		_ = userConn.Close()
		return nil, err
	}

	s := &Server{
		authConn:  authConn,
		userConn:  userConn,
		payConn:   payConn,
		auth:      authpb.NewAuthServiceClient(authConn),
		user:      userpb.NewUserServiceClient(userConn),
		payment:   paymentpb.NewPaymentServiceClient(payConn),
		mux:       http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/register", s.handleRegister)

	return s, nil
}

// Close releases gRPC connections.
func (s *Server) Close() error {
	var errs []error
	if s.authConn != nil {
		errs = append(errs, s.authConn.Close())
	}
	if s.userConn != nil {
		errs = append(errs, s.userConn.Close())
	}
	if s.payConn != nil {
		errs = append(errs, s.payConn.Close())
	}
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// RegisterRequest is the public JSON body for signup orchestration.
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	UserID       int64  `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	Message      string `json:"message"`
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.TrimSpace(body.Email)
	if body.Name == "" || body.Email == "" || body.Password == "" {
		http.Error(w, "name, email and password are required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	authResp, err := s.auth.Register(ctx, &authpb.RegisterRequest{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	userResp, err := s.user.CreateUser(ctx, &userpb.CreateUserRequest{
		Name:  body.Name,
		Email: body.Email,
	})
	if err != nil {
		log.Printf("gateway: CreateUser after Register failed: %v", err)
		if cerr := s.compensateAuth(ctx, body.Email); cerr != nil {
			log.Printf("gateway: compensate auth after CreateUser failure: %v", cerr)
		}
		writeGRPCError(w, err)
		return
	}

	payCtx := withBearer(ctx, authResp.Token)
	_, err = s.payment.CreateAccount(payCtx, &paymentpb.CreateAccountRequest{UserId: userResp.Id})
	if err != nil {
		log.Printf("gateway: CreateAccount failed (user exists, account missing): %v", err)
		if cerr := s.compensateUserAndAuth(ctx, authResp.Token, userResp.Id, body.Email); cerr != nil {
			log.Printf("gateway: compensation after CreateAccount failure: %v", cerr)
		}
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(registerResponse{
		UserID:       userResp.Id,
		Name:         userResp.Name,
		Email:        userResp.Email,
		Token:        authResp.Token,
		RefreshToken: authResp.RefreshToken,
		Message:      "registered with profile and payment account",
	})
}

func withBearer(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(ctx, md)
}

func (s *Server) compensateAuth(ctx context.Context, email string) error {
	_, err := s.auth.DeleteAuthUser(ctx, &authpb.DeleteAuthUserRequest{Email: email})
	return err
}

func (s *Server) compensateUserAndAuth(ctx context.Context, token string, userID int64, email string) error {
	var firstErr error

	userCtx := withBearer(ctx, token)
	if _, err := s.user.DeleteUser(userCtx, &userpb.DeleteUserRequest{Id: userID}); err != nil {
		firstErr = err
	}

	if _, err := s.auth.DeleteAuthUser(ctx, &authpb.DeleteAuthUserRequest{Email: email}); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	code := st.Code()
	switch code {
	case codes.InvalidArgument, codes.AlreadyExists, codes.FailedPrecondition:
		http.Error(w, st.Message(), http.StatusBadRequest)
	case codes.Unauthenticated:
		http.Error(w, st.Message(), http.StatusUnauthorized)
	case codes.NotFound:
		http.Error(w, st.Message(), http.StatusNotFound)
	case codes.PermissionDenied:
		http.Error(w, st.Message(), http.StatusForbidden)
	default:
		http.Error(w, st.Message(), http.StatusBadGateway)
	}
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// ListenAddr returns HTTP listen address from env HTTP_ADDR or default ":8080".
func ListenAddr() string {
	return getenv("HTTP_ADDR", ":8080")
}
