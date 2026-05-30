package repository

import (
	"errors"
	"strings"

	"github.com/basketikun/infinite-canvas/model"
	"gorm.io/gorm"
)

// ListUsers 分页查询用户。
func ListUsers(q model.Query) ([]model.User, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := db.Model(&model.User{})
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ? OR linux_do_id LIKE ?", like, like, like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []model.User
	err = tx.Order("created_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&users).Error
	return users, total, err
}

// CountUsers 返回用户总数。
func CountUsers() (int64, error) {
	db, err := DB()
	if err != nil {
		return 0, err
	}
	var total int64
	return total, db.Model(&model.User{}).Count(&total).Error
}

// HasAdmin 判断系统中是否存在管理员。
func HasAdmin() (bool, error) {
	db, err := DB()
	if err != nil {
		return false, err
	}
	var total int64
	err = db.Model(&model.User{}).Where("role = ?", model.UserRoleAdmin).Count(&total).Error
	return total > 0, err
}

// GetUserByID 根据 ID 查询用户。
func GetUserByID(id string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "id = ?", id)
}

// GetUserByUsername 根据用户名查询用户。
func GetUserByUsername(username string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "username = ?", username)
}

// SaveUser 保存用户信息。
func SaveUser(user model.User) (model.User, error) {
	db, err := DB()
	if err != nil {
		return user, err
	}
	return user, db.Save(&user).Error
}

func AdjustUserCreditsWithLog(id string, credits int, log model.CreditLog, now string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	user := model.User{}
	ok := false
	err = db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("id = ?", id).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		ok = true
		oldCredits := user.Credits
		user.Credits = credits
		user.UpdatedAt = now
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		if oldCredits == credits {
			return nil
		}
		log.UserID = user.ID
		log.Amount = credits - oldCredits
		log.Balance = credits
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		return nil
	})
	return user, ok, err
}

func ConsumeUserCredits(id string, credits int, now string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	if credits <= 0 {
		user, ok, err := GetUserByID(id)
		return user, ok, err
	}
	tx := db.Model(&model.User{}).Where("id = ? AND credits >= ?", id, credits).Updates(map[string]any{
		"credits":    gorm.Expr("credits - ?", credits),
		"updated_at": now,
	})
	if tx.Error != nil {
		return model.User{}, false, tx.Error
	}
	user, ok, err := GetUserByID(id)
	return user, ok && tx.RowsAffected > 0, err
}

func ConsumeUserCreditsWithLog(id string, credits int, log model.CreditLog, now string) (model.CreditLog, bool, error) {
	db, err := DB()
	if err != nil {
		return log, false, err
	}
	ok := false
	err = db.Transaction(func(tx *gorm.DB) error {
		if credits > 0 {
			result := tx.Model(&model.User{}).Where("id = ? AND credits >= ?", id, credits).Updates(map[string]any{
				"credits":    gorm.Expr("credits - ?", credits),
				"updated_at": now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
		}
		user := model.User{}
		if err := tx.Where("id = ?", id).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		log.Balance = user.Credits
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		ok = true
		return nil
	})
	return log, ok, err
}

func RefundUserCredits(id string, credits int, now string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	if credits <= 0 {
		user, ok, err := GetUserByID(id)
		return user, ok, err
	}
	tx := db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
		"credits":    gorm.Expr("credits + ?", credits),
		"updated_at": now,
	})
	if tx.Error != nil {
		return model.User{}, false, tx.Error
	}
	user, ok, err := GetUserByID(id)
	return user, ok && tx.RowsAffected > 0, err
}

func RefundUserCreditsWithLog(id string, credits int, log model.CreditLog, now string) (model.CreditLog, bool, error) {
	db, err := DB()
	if err != nil {
		return log, false, err
	}
	if credits <= 0 {
		return log, true, nil
	}
	ok := false
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{
			"credits":    gorm.Expr("credits + ?", credits),
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		user := model.User{}
		if err := tx.Where("id = ?", id).First(&user).Error; err != nil {
			return err
		}
		log.Balance = user.Credits
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		ok = true
		return nil
	})
	return log, ok, err
}

// SaveCreditLog 保存算力点变更流水。
func SaveCreditLog(log model.CreditLog) (model.CreditLog, error) {
	db, err := DB()
	if err != nil {
		return log, err
	}
	return log, db.Save(&log).Error
}

func GetCreditLogByID(id string) (model.CreditLog, bool, error) {
	db, err := DB()
	if err != nil {
		return model.CreditLog{}, false, err
	}
	log := model.CreditLog{}
	err = db.Where("id = ?", id).First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CreditLog{}, false, nil
	}
	return log, err == nil, err
}

func GetCreditLogByRelatedID(userID string, relatedID string, logType model.CreditLogType) (model.CreditLog, bool, error) {
	db, err := DB()
	if err != nil {
		return model.CreditLog{}, false, err
	}
	log := model.CreditLog{}
	err = db.Where("user_id = ? AND related_id = ? AND type = ?", userID, relatedID, logType).Order("created_at desc").First(&log).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.CreditLog{}, false, nil
	}
	return log, err == nil, err
}

func UpdateCreditLogTaskBinding(id string, relatedID string, extra string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	result := db.Model(&model.CreditLog{}).Where("id = ?", id).Updates(map[string]any{
		"related_id": relatedID,
		"extra":      extra,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListCreditLogs(q model.Query) ([]model.CreditLog, int64, error) {
	db, err := DB()
	if err != nil {
		return nil, 0, err
	}
	q.Normalize()
	tx := db.Model(&model.CreditLog{})
	if keyword := strings.TrimSpace(q.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("user_id LIKE ? OR type LIKE ? OR remark LIKE ? OR related_id LIKE ?", like, like, like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.CreditLog
	err = tx.Order("created_at desc").Offset(q.Offset()).Limit(q.PageSize).Find(&logs).Error
	return logs, total, err
}

func DeleteCreditLog(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.CreditLog{}, "id = ?", id).Error
}

// DeleteUser 删除指定用户。
func DeleteUser(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.User{}, "id = ?", id).Error
}

// GetUserByLinuxDoID 根据 Linux.do ID 查询用户。
func GetUserByLinuxDoID(id string) (model.User, bool, error) {
	db, err := DB()
	if err != nil {
		return model.User{}, false, err
	}
	return findUser(db, "linux_do_id = ?", id)
}

// findUser 查询单个用户，并将未命中转换为 ok=false。
func findUser(db *gorm.DB, query string, args ...any) (model.User, bool, error) {
	user := model.User{}
	err := db.Where(query, args...).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}
	return user, err == nil, err
}
