package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authpb "github.com/sonni-a/minibank/api/auth"
	paymentpb "github.com/sonni-a/minibank/api/payment"
	userpb "github.com/sonni-a/minibank/api/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubAuthClient struct {
	registerFn     func(context.Context, *authpb.RegisterRequest) (*authpb.AuthResponse, error)
	loginFn        func(context.Context, *authpb.LoginRequest) (*authpb.AuthResponse, error)
	refreshTokenFn func(context.Context, *authpb.RefreshTokenRequest) (*authpb.AuthResponse, error)
	deleteAuthFn   func(context.Context, *authpb.DeleteAuthUserRequest) (*authpb.DeleteAuthUserResponse, error)
}

func (s *stubAuthClient) Register(ctx context.Context, req *authpb.RegisterRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	if s.registerFn != nil {
		return s.registerFn(ctx, req)
	}
	panic("Register not stubbed")
}

func (s *stubAuthClient) Login(ctx context.Context, req *authpb.LoginRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	if s.loginFn != nil {
		return s.loginFn(ctx, req)
	}
	panic("Login not stubbed")
}

func (s *stubAuthClient) RefreshToken(ctx context.Context, req *authpb.RefreshTokenRequest, _ ...grpc.CallOption) (*authpb.AuthResponse, error) {
	if s.refreshTokenFn != nil {
		return s.refreshTokenFn(ctx, req)
	}
	panic("RefreshToken not stubbed")
}

func (s *stubAuthClient) DeleteAuthUser(ctx context.Context, req *authpb.DeleteAuthUserRequest, _ ...grpc.CallOption) (*authpb.DeleteAuthUserResponse, error) {
	if s.deleteAuthFn != nil {
		return s.deleteAuthFn(ctx, req)
	}
	return &authpb.DeleteAuthUserResponse{}, nil
}

type stubUserClient struct {
	createUserFn func(context.Context, *userpb.CreateUserRequest) (*userpb.UserResponse, error)
	getMyUserFn  func(context.Context, *userpb.GetMyUserRequest) (*userpb.UserResponse, error)
	updateUserFn func(context.Context, *userpb.UpdateUserRequest) (*userpb.UserResponse, error)
	deleteUserFn func(context.Context, *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error)
}

func (s *stubUserClient) CreateUser(ctx context.Context, req *userpb.CreateUserRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, req)
	}
	panic("CreateUser not stubbed")
}

func (s *stubUserClient) GetUser(context.Context, *userpb.GetUserRequest, ...grpc.CallOption) (*userpb.UserResponse, error) {
	panic("GetUser not stubbed")
}

func (s *stubUserClient) GetMyUser(ctx context.Context, req *userpb.GetMyUserRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if s.getMyUserFn != nil {
		return s.getMyUserFn(ctx, req)
	}
	panic("GetMyUser not stubbed")
}

func (s *stubUserClient) UpdateUser(ctx context.Context, req *userpb.UpdateUserRequest, _ ...grpc.CallOption) (*userpb.UserResponse, error) {
	if s.updateUserFn != nil {
		return s.updateUserFn(ctx, req)
	}
	panic("UpdateUser not stubbed")
}

func (s *stubUserClient) DeleteUser(ctx context.Context, req *userpb.DeleteUserRequest, _ ...grpc.CallOption) (*userpb.DeleteUserResponse, error) {
	if s.deleteUserFn != nil {
		return s.deleteUserFn(ctx, req)
	}
	panic("DeleteUser not stubbed")
}

type stubPaymentClient struct {
	createAccountFn func(context.Context, *paymentpb.CreateAccountRequest) (*paymentpb.AccountResponse, error)
	getBalanceFn    func(context.Context, *paymentpb.GetBalanceRequest) (*paymentpb.BalanceResponse, error)
	depositFn       func(context.Context, *paymentpb.DepositRequest) (*paymentpb.BalanceResponse, error)
	transferFn      func(context.Context, *paymentpb.TransferRequest) (*paymentpb.TransferResponse, error)
}

func (s *stubPaymentClient) CreateAccount(ctx context.Context, req *paymentpb.CreateAccountRequest, _ ...grpc.CallOption) (*paymentpb.AccountResponse, error) {
	if s.createAccountFn != nil {
		return s.createAccountFn(ctx, req)
	}
	panic("CreateAccount not stubbed")
}

func (s *stubPaymentClient) GetBalance(ctx context.Context, req *paymentpb.GetBalanceRequest, _ ...grpc.CallOption) (*paymentpb.BalanceResponse, error) {
	if s.getBalanceFn != nil {
		return s.getBalanceFn(ctx, req)
	}
	panic("GetBalance not stubbed")
}

func (s *stubPaymentClient) Deposit(ctx context.Context, req *paymentpb.DepositRequest, _ ...grpc.CallOption) (*paymentpb.BalanceResponse, error) {
	if s.depositFn != nil {
		return s.depositFn(ctx, req)
	}
	panic("Deposit not stubbed")
}

func (s *stubPaymentClient) Transfer(ctx context.Context, req *paymentpb.TransferRequest, _ ...grpc.CallOption) (*paymentpb.TransferResponse, error) {
	if s.transferFn != nil {
		return s.transferFn(ctx, req)
	}
	panic("Transfer not stubbed")
}

func newTestServer(auth authpb.AuthServiceClient, user userpb.UserServiceClient, payment paymentpb.PaymentServiceClient) *Server {
	s := &Server{auth: auth, user: user, payment: payment, mux: http.NewServeMux()}
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
	return s
}

func serve(s *Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

const (
	testEmail    = "user@example.com"
	testPassword = "password123"
	bearer       = "Bearer test-token"
)

func TestHealth(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	w := serve(s, http.MethodGet, "/health", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestTokenFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", bearer)

	token, ok := tokenFromRequest(req)
	if !ok || token != "test-token" {
		t.Fatalf("token = %q ok = %v", token, ok)
	}

	req.Header.Set("Authorization", "Basic xyz")
	if _, ok := tokenFromRequest(req); ok {
		t.Fatal("expected missing token for non-Bearer header")
	}
}

func TestRegister_InvalidRequest(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{`},
		{"invalid email", `{"name":"Alice","email":"bad","password":"password123"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(s, http.MethodPost, "/api/v1/register", tt.body, nil)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestRegister_Success(t *testing.T) {
	s := newTestServer(
		&stubAuthClient{registerFn: func(context.Context, *authpb.RegisterRequest) (*authpb.AuthResponse, error) {
			return &authpb.AuthResponse{Token: "access", RefreshToken: "refresh"}, nil
		}},
		&stubUserClient{createUserFn: func(_ context.Context, req *userpb.CreateUserRequest) (*userpb.UserResponse, error) {
			return &userpb.UserResponse{Id: 1, Name: req.Name, Email: req.Email}, nil
		}},
		&stubPaymentClient{createAccountFn: func(_ context.Context, req *paymentpb.CreateAccountRequest) (*paymentpb.AccountResponse, error) {
			if req.UserId != 1 {
				t.Fatalf("user_id = %d, want 1", req.UserId)
			}
			return &paymentpb.AccountResponse{UserId: 1}, nil
		}},
	)

	body := `{"name":"Alice","email":"` + testEmail + `","password":"` + testPassword + `"}`
	w := serve(s, http.MethodPost, "/api/v1/register", body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"user_id":1`) {
		t.Fatalf("body = %q", w.Body.String())
	}
}

func TestLogin_InvalidRequest(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	w := serve(s, http.MethodPost, "/api/v1/login", `{`, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLogin_GRPCError_ReturnsMappedStatus(t *testing.T) {
	s := newTestServer(
		&stubAuthClient{loginFn: func(context.Context, *authpb.LoginRequest) (*authpb.AuthResponse, error) {
			return nil, status.Error(codes.NotFound, "user not found")
		}},
		&stubUserClient{},
		&stubPaymentClient{},
	)

	body := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
	w := serve(s, http.MethodPost, "/api/v1/login", body, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRefresh_MissingToken_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	w := serve(s, http.MethodPost, "/api/v1/refresh", `{}`, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuthedRoutes_WithoutBearer_ReturnUnauthorized(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/me", ""},
		{http.MethodPut, "/api/v1/me", `{"name":"Alice","email":"` + testEmail + `"}`},
		{http.MethodDelete, "/api/v1/me", ""},
		{http.MethodGet, "/api/v1/balance", ""},
		{http.MethodPost, "/api/v1/deposit", `{"amount_minor":100}`},
		{http.MethodPost, "/api/v1/transfer", `{"to_user_id":2,"amount_minor":100}`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := serve(s, tt.method, tt.path, tt.body, nil)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestUpdateMe_InvalidEmail_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(
		&stubAuthClient{},
		&stubUserClient{getMyUserFn: func(context.Context, *userpb.GetMyUserRequest) (*userpb.UserResponse, error) {
			return &userpb.UserResponse{Id: 1, Email: testEmail}, nil
		}},
		&stubPaymentClient{},
	)

	w := serve(s, http.MethodPut, "/api/v1/me", `{"name":"Alice","email":"bad"}`, map[string]string{
		"Authorization": bearer,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestDeposit_InvalidAmount_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	w := serve(s, http.MethodPost, "/api/v1/deposit", `{"amount_minor":0}`, map[string]string{
		"Authorization": bearer,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTransfer_InvalidBody_ReturnsBadRequest(t *testing.T) {
	s := newTestServer(&stubAuthClient{}, &stubUserClient{}, &stubPaymentClient{})

	w := serve(s, http.MethodPost, "/api/v1/transfer", `{"to_user_id":0,"amount_minor":100}`, map[string]string{
		"Authorization": bearer,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
