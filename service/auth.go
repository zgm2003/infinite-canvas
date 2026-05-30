package service

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type TokenClaims struct {
	UserID   string         `json:"userId"`
	Username string         `json:"username"`
	Role     model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type userExtra struct {
	LinuxDo any `json:"linuxDo,omitempty"`
}

type creditLogExtra struct {
	Model          string `json:"model,omitempty"`
	Path           string `json:"path,omitempty"`
	TaskID         string `json:"taskId,omitempty"`
	ChannelName    string `json:"channelName,omitempty"`
	ChannelBaseURL string `json:"channelBaseUrl,omitempty"`
}

func EnsureDefaultAdmin() error {
	if strings.TrimSpace(config.Cfg.AdminUsername) == "" || strings.TrimSpace(config.Cfg.AdminPassword) == "" {
		return nil
	}
	WarnDefaultSecurityConfig()
	hasAdmin, err := repository.HasAdmin()
	if err != nil || hasAdmin {
		return err
	}
	hash, err := hashPassword(config.Cfg.AdminPassword)
	if err != nil {
		return err
	}
	_, err = repository.SaveUser(model.User{
		ID:        newID("user"),
		Username:  strings.TrimSpace(config.Cfg.AdminUsername),
		Password:  hash,
		Role:      model.UserRoleAdmin,
		AffCode:   newAffCode(),
		Status:    model.UserStatusActive,
		CreatedAt: now(),
		UpdatedAt: now(),
	})
	return err
}

func Register(username string, password string) (model.AuthSession, error) {
	settings, err := repository.GetSettings()
	if err != nil {
		return model.AuthSession{}, err
	}
	normalizedSettings := normalizeSettings(settings)
	if normalizedSettings.Public.Auth.AllowRegister != nil && !*normalizedSettings.Public.Auth.AllowRegister {
		return model.AuthSession{}, safeMessageError{message: "当前未开放注册", status: http.StatusForbidden}
	}
	username = strings.TrimSpace(username)
	if strings.ContainsAny(username, " \t\r\n") {
		return model.AuthSession{}, safeMessageError{message: "用户名不能包含空格", status: http.StatusBadRequest}
	}
	if username == "" || password == "" {
		return model.AuthSession{}, safeMessageError{message: "用户名和密码不能为空", status: http.StatusBadRequest}
	}
	if _, ok, err := repository.GetUserByUsername(username); err != nil || ok {
		if err != nil {
			return model.AuthSession{}, err
		}
		return model.AuthSession{}, safeMessageError{message: "用户名已存在", status: http.StatusConflict}
	}
	hash, err := hashPassword(password)
	if err != nil {
		return model.AuthSession{}, err
	}
	user, err := repository.SaveUser(model.User{
		ID:        newID("user"),
		Username:  username,
		Password:  hash,
		Role:      model.UserRoleUser,
		AffCode:   newAffCode(),
		Status:    model.UserStatusActive,
		CreatedAt: now(),
		UpdatedAt: now(),
	})
	if err != nil {
		return model.AuthSession{}, err
	}
	return newSession(user)
}

func Login(username string, password string) (model.AuthSession, error) {
	user, ok, err := repository.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return model.AuthSession{}, err
	}
	if !ok || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return model.AuthSession{}, safeMessageError{message: "用户名或密码错误", status: http.StatusUnauthorized}
	}
	if user.Status == model.UserStatusBan {
		return model.AuthSession{}, safeMessageError{message: "账号已被禁用", status: http.StatusForbidden}
	}
	normalizeUserDefaults(&user)
	user.LastLoginAt = now()
	user.UpdatedAt = now()
	user, err = repository.SaveUser(user)
	if err != nil {
		return model.AuthSession{}, err
	}
	return newSession(user)
}

func LinuxDoAuthorizeURL(r *http.Request, redirect string) (string, error) {
	settings, err := repository.GetSettings()
	if err != nil {
		return "", err
	}
	settings = normalizeSettings(settings)
	linuxDo := settings.Private.Auth.LinuxDo
	if !settings.Public.Auth.LinuxDo.Enabled {
		return "", safeMessageError{message: "Linux.do 登录未开启", status: http.StatusForbidden}
	}
	if strings.TrimSpace(linuxDo.ClientID) == "" || strings.TrimSpace(linuxDo.ClientSecret) == "" {
		return "", safeMessageError{message: "Linux.do 登录未配置", status: http.StatusBadRequest}
	}
	values := url.Values{}
	values.Set("client_id", linuxDo.ClientID)
	values.Set("redirect_uri", linuxDoRedirectURI(r))
	values.Set("response_type", "code")
	values.Set("scope", "read")
	values.Set("state", base64.RawURLEncoding.EncodeToString([]byte(redirect)))
	return config.Cfg.LinuxDoAuthorizeURL + "?" + values.Encode(), nil
}

func LoginWithLinuxDo(r *http.Request, code string, state string) (model.AuthSession, string, error) {
	redirect := decodeState(state)
	settings, err := repository.GetSettings()
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	settings = normalizeSettings(settings)
	linuxDo := settings.Private.Auth.LinuxDo
	if !settings.Public.Auth.LinuxDo.Enabled {
		return model.AuthSession{}, redirect, safeMessageError{message: "Linux.do 登录未开启", status: http.StatusForbidden}
	}
	if strings.TrimSpace(linuxDo.ClientID) == "" || strings.TrimSpace(linuxDo.ClientSecret) == "" {
		return model.AuthSession{}, redirect, safeMessageError{message: "Linux.do 登录未配置", status: http.StatusBadRequest}
	}
	token, err := linuxDoAccessToken(r, code, linuxDo)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	profile, err := linuxDoProfile(token)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	linuxDoID := fmt.Sprint(profile.ID)
	if strings.TrimSpace(linuxDoID) == "" || linuxDoID == "0" {
		return model.AuthSession{}, redirect, safeMessageError{message: "Linux.do 用户信息无效", status: http.StatusBadGateway}
	}
	user, ok, err := repository.GetUserByLinuxDoID(linuxDoID)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	if !ok {
		if settings.Public.Auth.AllowRegister != nil && !*settings.Public.Auth.AllowRegister {
			return model.AuthSession{}, redirect, safeMessageError{message: "当前未开放注册", status: http.StatusForbidden}
		}
		user = model.User{
			ID:          newID("user"),
			Username:    linuxDoUsername(profile.Username, linuxDoID),
			DisplayName: strings.TrimSpace(profile.Name),
			AvatarURL:   linuxDoAvatar(profile.AvatarTemplate),
			Role:        model.UserRoleUser,
			AffCode:     newAffCode(),
			LinuxDoID:   linuxDoID,
			Status:      model.UserStatusActive,
			CreatedAt:   now(),
		}
	} else if user.Status == model.UserStatusBan {
		return model.AuthSession{}, redirect, safeMessageError{message: "账号已被禁用", status: http.StatusForbidden}
	}
	user.DisplayName = firstNonEmpty(profile.Name, user.DisplayName)
	user.AvatarURL = firstNonEmpty(linuxDoAvatar(profile.AvatarTemplate), user.AvatarURL)
	user.LastLoginAt = now()
	user.UpdatedAt = now()
	extra, _ := json.Marshal(userExtra{LinuxDo: profile})
	user.Extra = string(extra)
	user, err = repository.SaveUser(user)
	if err != nil {
		return model.AuthSession{}, redirect, err
	}
	session, err := newSession(user)
	return session, redirect, err
}

func ParseToken(tokenText string) (TokenClaims, error) {
	claims := TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenText, &claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("登录状态无效")
		}
		return []byte(config.Cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return TokenClaims{}, errors.New("登录状态无效")
	}
	return claims, nil
}

func CurrentAuthUser(tokenText string) (model.AuthUser, bool) {
	claims, err := ParseToken(tokenText)
	if err != nil {
		return model.AuthUser{}, false
	}
	user, ok, err := repository.GetUserByID(claims.UserID)
	if err != nil || !ok {
		return model.AuthUser{}, false
	}
	if user.Status == model.UserStatusBan {
		return model.AuthUser{}, false
	}
	return model.PublicUser(user), true
}

func ListUsers(q model.Query) (model.UserList, error) {
	users, total, err := repository.ListUsers(q)
	if err != nil {
		return model.UserList{}, err
	}
	for i := range users {
		users[i].Password = ""
		normalizeUserDefaults(&users[i])
	}
	return model.UserList{Items: users, Total: int(total)}, nil
}

func SaveUser(user model.User, password string) (model.User, error) {
	user.Username = strings.TrimSpace(user.Username)
	if strings.ContainsAny(user.Username, " \t\r\n") {
		return user, safeMessageError{message: "用户名不能包含空格", status: http.StatusBadRequest}
	}
	if user.Username == "" {
		return user, safeMessageError{message: "用户名不能为空", status: http.StatusBadRequest}
	}
	if user.Role == "" || user.Role == model.UserRoleGuest {
		user.Role = model.UserRoleUser
	}
	if user.Status == "" {
		user.Status = model.UserStatusActive
	}
	if saved, ok, err := repository.GetUserByUsername(user.Username); err != nil {
		return user, err
	} else if ok && saved.ID != user.ID {
		return user, safeMessageError{message: "用户名已存在", status: http.StatusConflict}
	}
	isCreate := user.ID == ""
	if isCreate {
		user.ID = newID("user")
		user.AffCode = newAffCode()
		user.CreatedAt = now()
	} else if saved, ok, err := repository.GetUserByID(user.ID); err != nil {
		return user, err
	} else if ok {
		user.CreatedAt = saved.CreatedAt
		user.Password = saved.Password
		user.AvatarURL = saved.AvatarURL
		user.Credits = saved.Credits
		user.Extra = saved.Extra
		if user.AffCode == "" {
			user.AffCode = saved.AffCode
		}
		if user.AffCode == "" {
			user.AffCode = newAffCode()
		}
		if user.LinuxDoID == "" {
			user.LinuxDoID = saved.LinuxDoID
		}
		user.LastLoginAt = saved.LastLoginAt
	} else {
		return user, safeMessageError{message: "用户不存在", status: http.StatusNotFound}
	}
	if password != "" {
		hash, err := hashPassword(password)
		if err != nil {
			return user, err
		}
		user.Password = hash
	}
	if isCreate && user.Password == "" {
		return user, safeMessageError{message: "密码不能为空", status: http.StatusBadRequest}
	}
	user.UpdatedAt = now()
	user, err := repository.SaveUser(user)
	user.Password = ""
	return user, err
}

func AdjustUserCredits(id string, credits int) (model.User, error) {
	timestamp := now()
	user, ok, err := repository.AdjustUserCreditsWithLog(id, credits, model.CreditLog{
		ID:        newID("credit"),
		Type:      model.CreditLogTypeAdminAdjust,
		Remark:    "后台手动调整",
		CreatedAt: timestamp,
	}, timestamp)
	if err != nil {
		return user, err
	}
	if !ok {
		return user, safeMessageError{message: "用户不存在", status: http.StatusNotFound}
	}
	user.Password = ""
	return user, err
}

func ConsumeUserCredits(userID string, modelName string, credits int, path string) (model.CreditLog, error) {
	if credits < 0 {
		return model.CreditLog{}, safeMessageError{message: "算力点参数错误", status: http.StatusBadRequest}
	}
	if credits == 0 && path != "/videos" {
		return model.CreditLog{}, nil
	}
	timestamp := now()
	extra, _ := json.Marshal(creditLogExtra{Model: modelName, Path: path})
	log := model.CreditLog{
		ID:        newID("credit"),
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -credits,
		Remark:    "调用模型 " + modelName,
		Extra:     string(extra),
		CreatedAt: timestamp,
	}
	log, ok, err := repository.ConsumeUserCreditsWithLog(userID, credits, log, timestamp)
	if err != nil {
		return model.CreditLog{}, err
	}
	if !ok {
		return model.CreditLog{}, safeMessageError{message: "算力点不足", status: http.StatusPaymentRequired}
	}
	return log, err
}

func RefundUserCredits(userID string, modelName string, credits int, path string) error {
	if credits <= 0 {
		return nil
	}
	timestamp := now()
	extra, _ := json.Marshal(creditLogExtra{Model: modelName, Path: path})
	_, ok, err := repository.RefundUserCreditsWithLog(userID, credits, model.CreditLog{
		ID:        newID("credit"),
		UserID:    userID,
		Type:      model.CreditLogTypeAIRefund,
		Amount:    credits,
		Remark:    "模型调用失败返还 " + modelName,
		Extra:     string(extra),
		CreatedAt: timestamp,
	}, timestamp)
	if err != nil {
		return err
	}
	if !ok {
		return safeMessageError{message: "用户不存在", status: http.StatusNotFound}
	}
	return err
}

func RefundVideoTaskCredits(userID string, modelName string, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	consumeLog, ok, err := repository.GetCreditLogByRelatedID(userID, taskID, model.CreditLogTypeAIConsume)
	if err != nil || !ok {
		return err
	}
	if consumeLog.Amount >= 0 {
		return nil
	}
	if _, refunded, err := repository.GetCreditLogByRelatedID(userID, taskID, model.CreditLogTypeAIRefund); err != nil || refunded {
		return err
	}
	credits := -consumeLog.Amount
	if persistedModel := creditLogModel(consumeLog.Extra); persistedModel != "" {
		modelName = persistedModel
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = "grok-imagine-video"
	}
	timestamp := now()
	extra, _ := json.Marshal(creditLogExtra{Model: modelName, Path: "/videos", TaskID: taskID})
	refundLogID := videoRefundCreditLogID(userID, taskID)
	_, ok, err = repository.RefundUserCreditsWithLog(userID, credits, model.CreditLog{
		ID:        refundLogID,
		UserID:    userID,
		Type:      model.CreditLogTypeAIRefund,
		Amount:    credits,
		RelatedID: taskID,
		Remark:    "视频任务失败返还 " + modelName,
		Extra:     string(extra),
		CreatedAt: timestamp,
	}, timestamp)
	if err != nil {
		if _, exists, getErr := repository.GetCreditLogByID(refundLogID); getErr == nil && exists {
			return nil
		}
		return err
	}
	if !ok {
		return safeMessageError{message: "用户不存在", status: http.StatusNotFound}
	}
	return err
}

func BindVideoTaskCreditLog(log model.CreditLog, taskID string, channel model.ModelChannel) error {
	taskID = strings.TrimSpace(taskID)
	if strings.TrimSpace(log.ID) == "" || taskID == "" {
		return nil
	}
	extra := parseCreditLogExtra(log.Extra)
	extra.TaskID = taskID
	extra.ChannelName = strings.TrimSpace(channel.Name)
	extra.ChannelBaseURL = normalizeChannelBaseURL(channel.BaseURL)
	encoded, _ := json.Marshal(extra)
	return repository.UpdateCreditLogTaskBinding(log.ID, taskID, string(encoded))
}

func VideoTaskChannel(userID string, taskID string) (model.ModelChannel, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return model.ModelChannel{}, false, nil
	}
	consumeLog, ok, err := repository.GetCreditLogByRelatedID(userID, taskID, model.CreditLogTypeAIConsume)
	if err != nil || !ok {
		return model.ModelChannel{}, false, err
	}
	extra := parseCreditLogExtra(consumeLog.Extra)
	if strings.TrimSpace(extra.ChannelBaseURL) == "" {
		return model.ModelChannel{}, false, nil
	}
	settings, err := repository.GetSettings()
	if err != nil {
		return model.ModelChannel{}, false, err
	}
	matches := []model.ModelChannel{}
	boundName := strings.TrimSpace(extra.ChannelName)
	boundBaseURL := normalizeChannelBaseURL(extra.ChannelBaseURL)
	for _, channel := range normalizePrivateSetting(settings.Private).Channels {
		if !channel.Enabled || strings.TrimSpace(channel.APIKey) == "" || strings.TrimSpace(channel.BaseURL) == "" {
			continue
		}
		if normalizeChannelBaseURL(channel.BaseURL) != boundBaseURL {
			continue
		}
		if boundName != "" && strings.TrimSpace(channel.Name) == boundName {
			return channel, true, nil
		}
		matches = append(matches, channel)
	}
	if boundName != "" {
		return model.ModelChannel{}, false, errVideoTaskChannelUnavailable
	}
	if len(matches) > 0 {
		return matches[0], true, nil
	}
	return model.ModelChannel{}, false, errVideoTaskChannelUnavailable
}

func creditLogModel(extra string) string {
	return strings.TrimSpace(parseCreditLogExtra(extra).Model)
}

func parseCreditLogExtra(extra string) creditLogExtra {
	payload := creditLogExtra{}
	_ = json.Unmarshal([]byte(extra), &payload)
	return payload
}

func normalizeChannelBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return strings.TrimSuffix(baseURL, "/v1")
}

func videoRefundCreditLogID(userID string, taskID string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(taskID)))
	return fmt.Sprintf("credit_vrefund_%x", sum)
}

var errVideoTaskChannelUnavailable = errors.New("video task bound channel unavailable")

func ListCreditLogs(q model.Query) (model.CreditLogList, error) {
	logs, total, err := repository.ListCreditLogs(q)
	if err != nil {
		return model.CreditLogList{}, err
	}
	return model.CreditLogList{Items: logs, Total: int(total)}, nil
}

func SaveCreditLog(log model.CreditLog) (model.CreditLog, error) {
	if log.ID == "" {
		log.ID = newID("credit")
		log.CreatedAt = now()
	}
	return repository.SaveCreditLog(log)
}

func DeleteCreditLog(id string) error {
	return repository.DeleteCreditLog(id)
}

func DeleteUser(id string) error {
	return repository.DeleteUser(id)
}

func GuestUser() model.AuthUser {
	return model.AuthUser{ID: "", Username: "guest", Role: model.UserRoleGuest}
}

func newSession(user model.User) (model.AuthSession, error) {
	token, err := newToken(user)
	if err != nil {
		return model.AuthSession{}, err
	}
	return model.AuthSession{Token: token, User: model.PublicUser(user)}, nil
}

func newToken(user model.User) (string, error) {
	expireHours := config.Cfg.JWTExpireHours
	if expireHours <= 0 {
		expireHours = 168
	}
	claims := TokenClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.Cfg.JWTSecret))
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

func newID(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func newAffCode() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
}

func normalizeUserDefaults(user *model.User) {
	if user.Status == "" {
		user.Status = model.UserStatusActive
	}
	if user.AffCode == "" {
		user.AffCode = newAffCode()
	}
}

type linuxDoTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type linuxDoUserResponse struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	AvatarTemplate string `json:"avatar_template"`
}

func linuxDoAccessToken(r *http.Request, code string, setting model.PrivateLinuxDoAuthSetting) (string, error) {
	values := url.Values{}
	values.Set("client_id", setting.ClientID)
	values.Set("client_secret", setting.ClientSecret)
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", linuxDoRedirectURI(r))
	req, _ := http.NewRequest(http.MethodPost, config.Cfg.LinuxDoTokenURL, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var payload linuxDoTokenResponse
	if err := doLinuxDoJSON(req, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", safeMessageError{message: "Linux.do 登录失败", status: http.StatusBadGateway}
	}
	return payload.AccessToken, nil
}

func linuxDoRedirectURI(r *http.Request) string {
	return RequestOrigin(r) + "/api/auth/linux-do/callback"
}

func linuxDoProfile(token string) (linuxDoUserResponse, error) {
	req, _ := http.NewRequest(http.MethodGet, config.Cfg.LinuxDoUserInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	var payload linuxDoUserResponse
	err := doLinuxDoJSON(req, &payload)
	return payload, err
}

func doLinuxDoJSON(req *http.Request, payload any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return safeMessageError{message: "Linux.do 登录失败", status: http.StatusBadGateway}
	}
	return json.NewDecoder(bytes.NewReader(body)).Decode(payload)
}

func linuxDoUsername(username string, id string) string {
	base := strings.TrimSpace(username)
	if base == "" {
		base = "linuxdo-" + id
	}
	if _, ok, err := repository.GetUserByUsername(base); err != nil || !ok {
		return base
	}
	return base + "-" + id
}

func linuxDoAvatar(template string) string {
	if strings.TrimSpace(template) == "" {
		return ""
	}
	if strings.HasPrefix(template, "//") {
		template = "https:" + template
	}
	if strings.HasPrefix(template, "/") {
		template = "https://linux.do" + template
	}
	return strings.ReplaceAll(template, "{size}", "120")
}

func decodeState(state string) string {
	data, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "/"
	}
	redirect := string(data)
	if !strings.HasPrefix(redirect, "/") {
		return "/"
	}
	return redirect
}

func RequestOrigin(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
	}
	return proto + "://" + host
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func WarnDefaultSecurityConfig() {
	if config.Cfg.AdminUsername == "admin" && config.Cfg.AdminPassword == "infinite-canvas" {
		log.Println("WARNING: using default admin credentials, please set ADMIN_USERNAME and ADMIN_PASSWORD to safer values before deployment")
	}
}
