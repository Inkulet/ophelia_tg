package app

import "time"

// Пользовательские избранные
type UserFavorite struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    int64     `gorm:"index;uniqueIndex:idx_user_fav"`
	WomanID   uint      `gorm:"index;uniqueIndex:idx_user_fav"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Просмотры карточек
type UserView struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    int64     `gorm:"index"`
	WomanID   uint      `gorm:"index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Личные подписки на ежедневную карточку
type UserSubscription struct {
	UserID    int64     `gorm:"primaryKey"`
	IsActive  bool      `gorm:"default:false"`
	Time      string    `gorm:"default:'09:00'"`
	LastRun   time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// История изменений
type ChangeLog struct {
	ID        uint  `gorm:"primaryKey"`
	WomanID   uint  `gorm:"index"`
	UserID    int64 `gorm:"index"`
	Field     string
	OldValue  string    `gorm:"type:text"`
	NewValue  string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Логи рассылок
type BroadcastLog struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    int64  `gorm:"index"`
	Message   string `gorm:"type:text"`
	Total     int
	Success   int
	Fail      int
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Модераторы
type Moderator struct {
	UserID    int64     `gorm:"primaryKey"`
	Role      string    `gorm:"default:'moderator'"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Логи действий модераторов/админов
type ModAction struct {
	ID        uint  `gorm:"primaryKey"`
	UserID    int64 `gorm:"index"`
	Action    string
	TargetID  string
	Details   string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// Коллекции
type Collection struct {
	ID          uint `gorm:"primaryKey"`
	Name        string
	Description string   `gorm:"type:text"`
	Tags        []string `gorm:"serializer:json"`
	Field       string
	YearFrom    int
	YearTo      int
	IsPublished bool      `gorm:"default:true"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}
