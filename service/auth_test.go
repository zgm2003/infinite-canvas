package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/basketikun/infinite-canvas/model"
	"github.com/basketikun/infinite-canvas/repository"
)

func TestRefundVideoTaskCreditsIsIdempotent(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_1",
		Username:  "tester",
		Role:      model.UserRoleUser,
		Credits:   3,
		AffCode:   "aff_1",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_consume_1",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   3,
		RelatedID: "video_task_1",
		Extra:     `{"model":"grok-imagine-video","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	if err := RefundVideoTaskCredits(user.ID, "grok-imagine-video", "video_task_1"); err != nil {
		t.Fatal(err)
	}
	if err := RefundVideoTaskCredits(user.ID, "grok-imagine-video", "video_task_1"); err != nil {
		t.Fatal(err)
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 7 {
		t.Fatalf("expected one refund to restore credits to 7, got %d", refreshed.Credits)
	}
	logs, _, err := repository.ListCreditLogs(model.Query{Keyword: "video_task_1", PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	refunds := 0
	for _, item := range logs {
		if item.Type == model.CreditLogTypeAIRefund {
			refunds++
		}
	}
	if refunds != 1 {
		t.Fatalf("expected exactly one refund log, got %d", refunds)
	}
}

func TestRefundVideoTaskCreditsDoesNotDoubleApplyDuplicateRefundLog(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_duplicate_refund"
	user := model.User{
		ID:        "user_video_duplicate_refund",
		Username:  "video-duplicate-refund",
		Role:      model.UserRoleUser,
		Credits:   7,
		AffCode:   "aff_video_duplicate_refund",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_duplicate_consume",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   3,
		RelatedID: taskID,
		Extra:     `{"model":"grok-imagine-video","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        videoRefundCreditLogID(user.ID, taskID),
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIRefund,
		Amount:    4,
		Balance:   7,
		RelatedID: "other_task",
		Extra:     `{"model":"grok-imagine-video","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	if err := RefundVideoTaskCredits(user.ID, "grok-imagine-video", taskID); err != nil {
		t.Fatal(err)
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 7 {
		t.Fatalf("expected duplicate refund log to keep credits at 7, got %d", refreshed.Credits)
	}
}

func TestRefundVideoTaskCreditsUsesConsumeLogModel(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_model_truth"
	user := model.User{
		ID:        "user_video_model_truth",
		Username:  "video-model-truth",
		Role:      model.UserRoleUser,
		Credits:   6,
		AffCode:   "aff_video_model_truth",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_model_truth_consume",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"original-video-model","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	if err := RefundVideoTaskCredits(user.ID, "wrong-query-model", taskID); err != nil {
		t.Fatal(err)
	}

	refund, ok, err := repository.GetCreditLogByRelatedID(user.ID, taskID, model.CreditLogTypeAIRefund)
	if err != nil || !ok {
		t.Fatalf("expected refund log, ok=%v err=%v", ok, err)
	}
	if creditLogModel(refund.Extra) != "original-video-model" {
		t.Fatalf("expected refund model to come from consume log, got extra=%s", refund.Extra)
	}
}

func TestVideoTaskChannelMatchesBoundNameWhenBaseURLIsShared(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_shared_base_url"
	const sharedBaseURL = "https://api.example.invalid"
	const userID = "user_shared_base_url"
	if _, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "wrong", BaseURL: sharedBaseURL, APIKey: "wrong-key", Enabled: true, Weight: 1},
			{Name: "correct", BaseURL: sharedBaseURL + "/", APIKey: "correct-key", Enabled: true, Weight: 1},
		}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_shared_base_url",
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"grok-imagine-video","path":"/videos","taskId":"` + taskID + `","channelName":"correct","channelBaseUrl":"` + sharedBaseURL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	channel, ok, err := VideoTaskChannel(userID, taskID)
	if err != nil || !ok {
		t.Fatalf("expected bound channel, ok=%v err=%v", ok, err)
	}
	if channel.APIKey != "correct-key" {
		t.Fatalf("expected exact bound channel API key, got %q", channel.APIKey)
	}
}

func TestVideoTaskChannelMatchesEquivalentV1BaseURL(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_equivalent_v1_base_url"
	const userID = "user_equivalent_v1_base_url"
	if _, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "video", BaseURL: "https://api.example.invalid/v1", APIKey: "video-key", Enabled: true, Weight: 1},
		}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_equivalent_v1_base_url",
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"grok-imagine-video","path":"/videos","taskId":"` + taskID + `","channelName":"video","channelBaseUrl":"https://api.example.invalid"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	channel, ok, err := VideoTaskChannel(userID, taskID)
	if err != nil || !ok {
		t.Fatalf("expected equivalent /v1 base URL to match bound channel, ok=%v err=%v", ok, err)
	}
	if channel.APIKey != "video-key" {
		t.Fatalf("expected bound channel API key, got %q", channel.APIKey)
	}
}

func TestVideoTaskChannelFailsWhenBoundNameIsMissingFromSharedBaseURL(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_missing_bound_name"
	const sharedBaseURL = "https://api.example.invalid"
	const userID = "user_missing_bound_name"
	if _, err := repository.SaveSettings(model.Settings{
		Private: model.PrivateSetting{Channels: []model.ModelChannel{
			{Name: "wrong", BaseURL: sharedBaseURL, APIKey: "wrong-key", Enabled: true, Weight: 1},
		}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_missing_bound_name",
		UserID:    userID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   6,
		RelatedID: taskID,
		Extra:     `{"model":"grok-imagine-video","path":"/videos","taskId":"` + taskID + `","channelName":"correct","channelBaseUrl":"` + sharedBaseURL + `"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	_, ok, err := VideoTaskChannel(userID, taskID)
	if err == nil || ok {
		t.Fatalf("expected stale bound channel to fail closed, ok=%v err=%v", ok, err)
	}
}

func TestBindVideoTaskCreditLogFailsWhenConsumeLogIsMissing(t *testing.T) {
	setupServiceTestDB(t)

	err := BindVideoTaskCreditLog(model.CreditLog{ID: "missing-credit-log"}, "task-1", model.ModelChannel{Name: "channel", BaseURL: "https://api.example.invalid"})
	if err == nil {
		t.Fatal("expected missing consume log binding to fail")
	}
}

func TestLoginWrongPasswordCarriesUnauthorizedStatus(t *testing.T) {
	setupServiceTestDB(t)
	username := "wrong-password-status-user"
	if _, err := SaveUser(model.User{
		Username: username,
		Role:     model.UserRoleUser,
		Status:   model.UserStatusActive,
	}, "correct-password"); err != nil {
		t.Fatal(err)
	}

	_, err := Login(username, "wrong-password")
	if err == nil {
		t.Fatal("expected wrong password to fail")
	}
	statusErr, ok := err.(interface{ StatusCode() int })
	if !ok {
		t.Fatalf("expected typed status error, got %T", err)
	}
	if statusErr.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401 status, got %d", statusErr.StatusCode())
	}
}

func TestRegisterValidationCarriesHTTPStatus(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := Register("bad name", "password"); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected username with spaces to carry HTTP 400, got %T %v", err, err)
	}

	username := "duplicate-register-status-user"
	if _, err := Register(username, "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := Register(username, "password"); statusCode(err) != http.StatusConflict {
		t.Fatalf("expected duplicate username to carry HTTP 409, got %T %v", err, err)
	}
}

func TestLinuxDoValidationCarriesHTTPStatus(t *testing.T) {
	setupServiceTestDB(t)
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/auth/linux-do/authorize", nil)

	if _, err := LinuxDoAuthorizeURL(request, ""); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected disabled Linux.do authorize to carry HTTP 403, got %T %v", err, err)
	}
	if _, _, err := LoginWithLinuxDo(request, "code", ""); statusCode(err) != http.StatusForbidden {
		t.Fatalf("expected disabled Linux.do callback to carry HTTP 403, got %T %v", err, err)
	}

	if _, err := repository.SaveSettings(model.Settings{
		Public: model.PublicSetting{Auth: model.PublicAuthSetting{LinuxDo: model.PublicLinuxDoAuthSetting{Enabled: true}}},
	}, "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := LinuxDoAuthorizeURL(request, ""); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing Linux.do client config to carry HTTP 400, got %T %v", err, err)
	}
	if _, _, err := LoginWithLinuxDo(request, "code", ""); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected missing Linux.do callback client config to carry HTTP 400, got %T %v", err, err)
	}
}

func TestSaveUserValidationCarriesHTTPStatus(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := SaveUser(model.User{Username: "bad name"}, "password"); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected username with spaces to carry HTTP 400, got %T %v", err, err)
	}
	username := "duplicate-save-user-status"
	if _, err := SaveUser(model.User{Username: username}, "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveUser(model.User{Username: username}, "password"); statusCode(err) != http.StatusConflict {
		t.Fatalf("expected duplicate username to carry HTTP 409, got %T %v", err, err)
	}
	if _, err := SaveUser(model.User{ID: "missing-user-id", Username: "missing-user"}, ""); statusCode(err) != http.StatusNotFound {
		t.Fatalf("expected missing user update to carry HTTP 404, got %T %v", err, err)
	}
}

func statusCode(err error) int {
	statusErr, ok := err.(interface{ StatusCode() int })
	if !ok {
		return 0
	}
	return statusErr.StatusCode()
}

func TestConsumeUserCreditsInsufficientCreditsCarriesPaymentRequiredStatus(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_consume_insufficient_status",
		Username:  "consume-insufficient-status",
		Role:      model.UserRoleUser,
		Credits:   1,
		AffCode:   "aff_consume_insufficient_status",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}

	if _, err := ConsumeUserCredits(user.ID, "grok-imagine-video", 4, "/videos"); statusCode(err) != http.StatusPaymentRequired {
		t.Fatalf("expected insufficient credits to carry HTTP 402, got %T %v", err, err)
	}
}

func TestConsumeUserCreditsRejectsNegativeCredits(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := ConsumeUserCredits("user_negative_credits", "gpt-test", -1, "/images/generations"); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected negative credit consume to carry HTTP 400, got %T %v", err, err)
	}
	if _, err := ConsumeUserCredits("user_negative_video_credits", "grok-imagine-video", -1, "/videos"); statusCode(err) != http.StatusBadRequest {
		t.Fatalf("expected negative video credit consume to carry HTTP 400, got %T %v", err, err)
	}
}

func TestRefundMissingUserCarriesNotFoundStatus(t *testing.T) {
	setupServiceTestDB(t)

	if err := RefundUserCredits("missing-user", "gpt-test", 1, "/chat/completions"); statusCode(err) != http.StatusNotFound {
		t.Fatalf("expected missing user refund to carry HTTP 404, got %T %v", err, err)
	}
}

func TestRefundVideoTaskMissingUserCarriesNotFoundStatus(t *testing.T) {
	setupServiceTestDB(t)
	const taskID = "video_task_missing_refund_user"
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_missing_refund_user",
		UserID:    "missing-video-user",
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -2,
		Balance:   0,
		RelatedID: taskID,
		Extra:     `{"model":"grok-imagine-video","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}

	if err := RefundVideoTaskCredits("missing-video-user", "grok-imagine-video", taskID); statusCode(err) != http.StatusNotFound {
		t.Fatalf("expected missing video refund user to carry HTTP 404, got %T %v", err, err)
	}
}

func TestAdjustUserCreditsRollsBackWhenCreditLogFails(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_admin_adjust_rollback",
		Username:  "admin-adjust-rollback",
		Role:      model.UserRoleUser,
		Credits:   5,
		AffCode:   "aff_admin_adjust_rollback",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	const triggerName = "block_admin_adjust_credit_log_insert"
	_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := db.Exec("CREATE TRIGGER " + triggerName + " BEFORE INSERT ON credit_logs WHEN NEW.type = 'admin_adjust' BEGIN SELECT RAISE(ABORT, 'admin adjust credit log insert blocked'); END;").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	})

	if _, err := AdjustUserCredits(user.ID, 13); err == nil {
		t.Fatal("expected admin adjust credit log insert failure")
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 5 {
		t.Fatalf("expected failed admin adjust log insert to roll back balance to 5, got %d", refreshed.Credits)
	}
}

func TestConsumeUserCreditsRollsBackWhenCreditLogFails(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_consume_rollback",
		Username:  "consume-rollback",
		Role:      model.UserRoleUser,
		Credits:   10,
		AffCode:   "aff_consume_rollback",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	const triggerName = "block_credit_log_insert"
	_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := db.Exec("CREATE TRIGGER " + triggerName + " BEFORE INSERT ON credit_logs BEGIN SELECT RAISE(ABORT, 'credit log insert blocked'); END;").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	})

	if _, err := ConsumeUserCredits(user.ID, "grok-imagine-video", 4, "/videos"); err == nil {
		t.Fatal("expected credit log insert failure")
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 10 {
		t.Fatalf("expected failed credit log insert to roll back balance to 10, got %d", refreshed.Credits)
	}
}

func TestRefundUserCreditsRollsBackWhenCreditLogFails(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_refund_rollback",
		Username:  "refund-rollback",
		Role:      model.UserRoleUser,
		Credits:   2,
		AffCode:   "aff_refund_rollback",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	const triggerName = "block_refund_credit_log_insert"
	_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := db.Exec("CREATE TRIGGER " + triggerName + " BEFORE INSERT ON credit_logs BEGIN SELECT RAISE(ABORT, 'refund credit log insert blocked'); END;").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	})

	if err := RefundUserCredits(user.ID, "grok-imagine-video", 4, "/images"); err == nil {
		t.Fatal("expected refund credit log insert failure")
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 2 {
		t.Fatalf("expected failed refund log insert to roll back balance to 2, got %d", refreshed.Credits)
	}
}

func TestRefundVideoTaskCreditsRollsBackWhenCreditLogFails(t *testing.T) {
	setupServiceTestDB(t)
	user := model.User{
		ID:        "user_video_refund_rollback",
		Username:  "video-refund-rollback",
		Role:      model.UserRoleUser,
		Credits:   3,
		AffCode:   "aff_video_refund_rollback",
		Status:    model.UserStatusActive,
		CreatedAt: "now",
		UpdatedAt: "now",
	}
	if _, err := repository.SaveUser(user); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SaveCreditLog(model.CreditLog{
		ID:        "credit_video_consume_rollback",
		UserID:    user.ID,
		Type:      model.CreditLogTypeAIConsume,
		Amount:    -4,
		Balance:   3,
		RelatedID: "video_task_rollback",
		Extra:     `{"model":"grok-imagine-video","path":"/videos"}`,
		CreatedAt: "now",
	}); err != nil {
		t.Fatal(err)
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	const triggerName = "block_video_refund_credit_log_insert"
	_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := db.Exec("CREATE TRIGGER " + triggerName + " BEFORE INSERT ON credit_logs BEGIN SELECT RAISE(ABORT, 'video refund credit log insert blocked'); END;").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	})

	if err := RefundVideoTaskCredits(user.ID, "grok-imagine-video", "video_task_rollback"); err == nil {
		t.Fatal("expected video refund credit log insert failure")
	}

	refreshed, ok, err := repository.GetUserByID(user.ID)
	if err != nil || !ok {
		t.Fatalf("expected user, ok=%v err=%v", ok, err)
	}
	if refreshed.Credits != 3 {
		t.Fatalf("expected failed video refund log insert to roll back balance to 3, got %d", refreshed.Credits)
	}
}
