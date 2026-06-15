package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	authpb "github.com/sonni-a/minibank/api/auth"
	paymentpb "github.com/sonni-a/minibank/api/payment"
	userpb "github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/pkg/env"
	"github.com/sonni-a/minibank/pkg/validate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Server struct {
	authConn *grpc.ClientConn
	userConn *grpc.ClientConn
	payConn  *grpc.ClientConn
	auth     authpb.AuthServiceClient
	user     userpb.UserServiceClient
	payment  paymentpb.PaymentServiceClient
	mux      *http.ServeMux
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
		authConn: authConn,
		userConn: userConn,
		payConn:  payConn,
		auth:     authpb.NewAuthServiceClient(authConn),
		user:     userpb.NewUserServiceClient(userConn),
		payment:  paymentpb.NewPaymentServiceClient(payConn),
		mux:      http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/refresh", s.handleRefresh)
	s.mux.HandleFunc("GET /api/v1/me", s.handleGetMe)
	s.mux.HandleFunc("PUT /api/v1/me", s.handleUpdateMe)
	s.mux.HandleFunc("DELETE /api/v1/me", s.handleDeleteMe)
	s.mux.HandleFunc("GET /api/v1/balance", s.handleGetBalance)
	s.mux.HandleFunc("POST /api/v1/deposit", s.handleDeposit)
	s.mux.HandleFunc("POST /api/v1/transfer", s.handleTransfer)

	return s, nil
}

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

func ListenAddr() string {
	return env.Getenv("HTTP_ADDR", ":8080")
}

func tokenFromRequest(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	return strings.TrimPrefix(h, "Bearer "), true
}

func authedCtx(w http.ResponseWriter, r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc, bool) {
	token, ok := tokenFromRequest(r)
	if !ok {
		http.Error(w, "authorization header missing or invalid", http.StatusUnauthorized)
		return nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	return withBearer(ctx, token), cancel, true
}

func withBearer(ctx context.Context, token string) context.Context {
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(ctx, md)
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
	if !validate.Email(body.Email) {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}
	if !validate.Password(body.Password) {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	authCtx, cancelAuth := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancelAuth()
	authResp, err := s.auth.Register(authCtx, &authpb.RegisterRequest{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	userCtx, cancelUser := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancelUser()
	userResp, err := s.user.CreateUser(userCtx, &userpb.CreateUserRequest{
		Name:  body.Name,
		Email: body.Email,
	})
	if err != nil {
		slog.Error("CreateUser after Register failed", "error", err)
		compCtx, compCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer compCancel()
		if cerr := s.compensateAuth(compCtx, body.Email); cerr != nil {
			slog.Error("compensate auth after CreateUser failure", "error", cerr)
		}
		writeGRPCError(w, err)
		return
	}

	payRPCctx, cancelPay := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancelPay()
	payCtx := withBearer(payRPCctx, authResp.Token)
	_, err = s.payment.CreateAccount(payCtx, &paymentpb.CreateAccountRequest{UserId: userResp.Id})
	if err != nil {
		slog.Error("CreateAccount failed, user exists but account missing", "error", err)
		compCtx, compCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer compCancel()
		if cerr := s.compensateUserAndAuth(compCtx, authResp.Token, userResp.Id, body.Email); cerr != nil {
			slog.Error("compensation after CreateAccount failure", "error", cerr)
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
	switch st.Code() {
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

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Email = strings.TrimSpace(body.Email)
	if body.Email == "" || body.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}
	if !validate.Email(body.Email) {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	resp, err := s.auth.Login(ctx, &authpb.LoginRequest{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.RefreshToken == "" {
		http.Error(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	resp, err := s.auth.RefreshToken(ctx, &authpb.RefreshTokenRequest{
		RefreshToken: body.RefreshToken,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	resp, err := s.user.GetMyUser(ctx, &userpb.GetMyUserRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	meResp, err := s.user.GetMyUser(ctx, &userpb.GetMyUserRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.TrimSpace(body.Email)
	if body.Name == "" || body.Email == "" {
		http.Error(w, "name and email are required", http.StatusBadRequest)
		return
	}
	if !validate.Email(body.Email) {
		http.Error(w, "invalid email format", http.StatusBadRequest)
		return
	}

	resp, err := s.user.UpdateUser(ctx, &userpb.UpdateUserRequest{
		Id:    meResp.Id,
		Name:  body.Name,
		Email: body.Email,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDeleteMe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	meResp, err := s.user.GetMyUser(ctx, &userpb.GetMyUserRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	resp, err := s.user.DeleteUser(ctx, &userpb.DeleteUserRequest{Id: meResp.Id})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	resp, err := s.payment.GetBalance(ctx, &paymentpb.GetBalanceRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	var body struct {
		AmountMinor int64 `json:"amount_minor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.AmountMinor <= 0 {
		http.Error(w, "amount_minor must be positive", http.StatusBadRequest)
		return
	}

	resp, err := s.payment.Deposit(ctx, &paymentpb.DepositRequest{AmountMinor: body.AmountMinor})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := authedCtx(w, r, 10*time.Second)
	if !ok {
		return
	}
	defer cancel()

	var body struct {
		ToUserID    int64 `json:"to_user_id"`
		AmountMinor int64 `json:"amount_minor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.ToUserID <= 0 {
		http.Error(w, "to_user_id is required", http.StatusBadRequest)
		return
	}
	if body.AmountMinor <= 0 {
		http.Error(w, "amount_minor must be positive", http.StatusBadRequest)
		return
	}

	resp, err := s.payment.Transfer(ctx, &paymentpb.TransferRequest{
		ToUserId:    body.ToUserID,
		AmountMinor: body.AmountMinor,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
