package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCMSNotFound = errors.New("cms entity not found")
	ErrEventIsFull = errors.New("event has reached max participants")
)

type Post struct {
	ID        string    `json:"id" gorm:"type:text;primaryKey" bson:"_id,omitempty"`
	Title     string    `json:"title" gorm:"size:255;not null" bson:"title"`
	Content   string    `json:"content" gorm:"type:text;not null" bson:"content"`
	MediaPath string    `json:"media_path" gorm:"type:text" bson:"media_path"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime" bson:"created_at"`
	IsHidden  bool      `json:"is_hidden" gorm:"default:false;index" bson:"is_hidden"`
}

type Event struct {
	ID                  string    `json:"id" gorm:"type:text;primaryKey" bson:"_id,omitempty"`
	Title               string    `json:"title" gorm:"size:255;not null" bson:"title"`
	Description         string    `json:"description" gorm:"type:text" bson:"description"`
	Date                time.Time `json:"date" gorm:"index;not null" bson:"date"`
	MaxParticipants     int       `json:"max_participants" gorm:"not null;default:0" bson:"max_participants"`
	CurrentParticipants []int64   `json:"current_participants" gorm:"serializer:json" bson:"current_participants"`
	MediaPath           string    `json:"media_path" gorm:"type:text" bson:"media_path"`
}

type Repository interface {
	InitPostgreSQL(ctx context.Context) error
	InitMongoDB(ctx context.Context) error

	CreatePost(ctx context.Context, post *Post) error
	GetPostByID(ctx context.Context, id string) (*Post, error)
	ListPosts(ctx context.Context, includeHidden bool) ([]Post, error)
	UpdatePost(ctx context.Context, post *Post) error
	DeletePost(ctx context.Context, id string) error

	CreateEvent(ctx context.Context, event *Event) error
	GetEventByID(ctx context.Context, id string) (*Event, error)
	ListEvents(ctx context.Context) ([]Event, error)
	UpdateEvent(ctx context.Context, event *Event) error
	DeleteEvent(ctx context.Context, id string) error
	AddEventParticipant(ctx context.Context, eventID string, userID int64) error
	RemoveEventParticipant(ctx context.Context, eventID string, userID int64) error
}

type PostgreSQLRepository struct {
	db *gorm.DB
}

func NewPostgreSQLRepository(db *gorm.DB) *PostgreSQLRepository {
	return &PostgreSQLRepository{db: db}
}

func (r *PostgreSQLRepository) InitPostgreSQL(ctx context.Context) error {
	if r.db == nil {
		return errors.New("postgres repository is not initialized")
	}

	db := r.db.WithContext(ctx)
	if err := db.AutoMigrate(&Post{}, &Event{}); err != nil {
		return fmt.Errorf("auto migrate cms models: %w", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC)").Error; err != nil {
		return fmt.Errorf("create posts index: %w", err)
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_events_date ON events(date)").Error; err != nil {
		return fmt.Errorf("create events index: %w", err)
	}

	return nil
}

func (r *PostgreSQLRepository) InitMongoDB(_ context.Context) error {
	return errors.New("mongodb init is not supported by PostgreSQLRepository")
}

func (r *PostgreSQLRepository) CreatePost(ctx context.Context, post *Post) error {
	if post == nil {
		return errors.New("post is nil")
	}
	ensurePostDefaults(post)
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *PostgreSQLRepository) GetPostByID(ctx context.Context, id string) (*Post, error) {
	var post Post
	if err := r.db.WithContext(ctx).First(&post, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMSNotFound
		}
		return nil, err
	}
	return &post, nil
}

func (r *PostgreSQLRepository) ListPosts(ctx context.Context, includeHidden bool) ([]Post, error) {
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if !includeHidden {
		q = q.Where("is_hidden = ?", false)
	}

	var posts []Post
	if err := q.Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostgreSQLRepository) UpdatePost(ctx context.Context, post *Post) error {
	if post == nil {
		return errors.New("post is nil")
	}
	if post.ID == "" {
		return errors.New("post id is required")
	}

	res := r.db.WithContext(ctx).Model(&Post{}).Where("id = ?", post.ID).Updates(map[string]any{
		"title":      post.Title,
		"content":    post.Content,
		"media_path": post.MediaPath,
		"is_hidden":  post.IsHidden,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *PostgreSQLRepository) DeletePost(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&Post{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *PostgreSQLRepository) CreateEvent(ctx context.Context, event *Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	ensureEventDefaults(event)
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *PostgreSQLRepository) GetEventByID(ctx context.Context, id string) (*Event, error) {
	var event Event
	if err := r.db.WithContext(ctx).First(&event, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCMSNotFound
		}
		return nil, err
	}
	return &event, nil
}

func (r *PostgreSQLRepository) ListEvents(ctx context.Context) ([]Event, error) {
	var events []Event
	if err := r.db.WithContext(ctx).Order("date ASC").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r *PostgreSQLRepository) UpdateEvent(ctx context.Context, event *Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	if event.ID == "" {
		return errors.New("event id is required")
	}
	ensureEventDefaults(event)

	res := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", event.ID).Updates(map[string]any{
		"title":                event.Title,
		"description":          event.Description,
		"date":                 event.Date,
		"max_participants":     event.MaxParticipants,
		"current_participants": event.CurrentParticipants,
		"media_path":           event.MediaPath,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *PostgreSQLRepository) DeleteEvent(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&Event{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *PostgreSQLRepository) AddEventParticipant(ctx context.Context, eventID string, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event Event
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, "id = ?", eventID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCMSNotFound
			}
			return err
		}

		if containsParticipant(event.CurrentParticipants, userID) {
			return nil
		}
		if event.MaxParticipants > 0 && len(event.CurrentParticipants) >= event.MaxParticipants {
			return ErrEventIsFull
		}

		event.CurrentParticipants = append(event.CurrentParticipants, userID)
		return tx.Model(&Event{}).Where("id = ?", eventID).
			Update("current_participants", event.CurrentParticipants).Error
	})
}

func (r *PostgreSQLRepository) RemoveEventParticipant(ctx context.Context, eventID string, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event Event
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, "id = ?", eventID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCMSNotFound
			}
			return err
		}

		event.CurrentParticipants = removeParticipant(event.CurrentParticipants, userID)
		return tx.Model(&Event{}).Where("id = ?", eventID).
			Update("current_participants", event.CurrentParticipants).Error
	})
}

type MongoRepository struct {
	db     *mongo.Database
	posts  *mongo.Collection
	events *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	repo := &MongoRepository{db: db}
	if db != nil {
		repo.posts = db.Collection("posts")
		repo.events = db.Collection("events")
	}
	return repo
}

func (r *MongoRepository) InitPostgreSQL(_ context.Context) error {
	return errors.New("postgres init is not supported by MongoRepository")
}

func (r *MongoRepository) InitMongoDB(ctx context.Context) error {
	if r.db == nil {
		return errors.New("mongo repository is not initialized")
	}

	names, err := r.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("list mongo collections: %w", err)
	}

	exists := make(map[string]struct{}, len(names))
	for _, name := range names {
		exists[name] = struct{}{}
	}

	if _, ok := exists["posts"]; !ok {
		if err := r.db.CreateCollection(ctx, "posts"); err != nil {
			return fmt.Errorf("create posts collection: %w", err)
		}
	}
	if _, ok := exists["events"]; !ok {
		if err := r.db.CreateCollection(ctx, "events"); err != nil {
			return fmt.Errorf("create events collection: %w", err)
		}
	}

	r.posts = r.db.Collection("posts")
	r.events = r.db.Collection("events")

	_, err = r.posts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "is_hidden", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("create posts indexes: %w", err)
	}

	_, err = r.events.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "date", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("create events indexes: %w", err)
	}

	return nil
}

func (r *MongoRepository) CreatePost(ctx context.Context, post *Post) error {
	if post == nil {
		return errors.New("post is nil")
	}
	ensurePostDefaults(post)

	coll := r.postsCollection()
	if coll == nil {
		return errors.New("posts collection is not initialized")
	}
	_, err := coll.InsertOne(ctx, post)
	return err
}

func (r *MongoRepository) GetPostByID(ctx context.Context, id string) (*Post, error) {
	coll := r.postsCollection()
	if coll == nil {
		return nil, errors.New("posts collection is not initialized")
	}

	var post Post
	err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&post)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCMSNotFound
		}
		return nil, err
	}
	return &post, nil
}

func (r *MongoRepository) ListPosts(ctx context.Context, includeHidden bool) ([]Post, error) {
	coll := r.postsCollection()
	if coll == nil {
		return nil, errors.New("posts collection is not initialized")
	}

	filter := bson.M{}
	if !includeHidden {
		filter["is_hidden"] = false
	}

	cur, err := coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var posts []Post
	if err := cur.All(ctx, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *MongoRepository) UpdatePost(ctx context.Context, post *Post) error {
	if post == nil {
		return errors.New("post is nil")
	}
	if post.ID == "" {
		return errors.New("post id is required")
	}

	coll := r.postsCollection()
	if coll == nil {
		return errors.New("posts collection is not initialized")
	}

	res, err := coll.ReplaceOne(ctx, bson.M{"_id": post.ID}, post)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) DeletePost(ctx context.Context, id string) error {
	coll := r.postsCollection()
	if coll == nil {
		return errors.New("posts collection is not initialized")
	}

	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) CreateEvent(ctx context.Context, event *Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	ensureEventDefaults(event)

	coll := r.eventsCollection()
	if coll == nil {
		return errors.New("events collection is not initialized")
	}
	_, err := coll.InsertOne(ctx, event)
	return err
}

func (r *MongoRepository) GetEventByID(ctx context.Context, id string) (*Event, error) {
	coll := r.eventsCollection()
	if coll == nil {
		return nil, errors.New("events collection is not initialized")
	}

	var event Event
	err := coll.FindOne(ctx, bson.M{"_id": id}).Decode(&event)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCMSNotFound
		}
		return nil, err
	}
	return &event, nil
}

func (r *MongoRepository) ListEvents(ctx context.Context) ([]Event, error) {
	coll := r.eventsCollection()
	if coll == nil {
		return nil, errors.New("events collection is not initialized")
	}

	cur, err := coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []Event
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *MongoRepository) UpdateEvent(ctx context.Context, event *Event) error {
	if event == nil {
		return errors.New("event is nil")
	}
	if event.ID == "" {
		return errors.New("event id is required")
	}
	ensureEventDefaults(event)

	coll := r.eventsCollection()
	if coll == nil {
		return errors.New("events collection is not initialized")
	}

	res, err := coll.ReplaceOne(ctx, bson.M{"_id": event.ID}, event)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) DeleteEvent(ctx context.Context, id string) error {
	coll := r.eventsCollection()
	if coll == nil {
		return errors.New("events collection is not initialized")
	}

	res, err := coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) AddEventParticipant(ctx context.Context, eventID string, userID int64) error {
	coll := r.eventsCollection()
	if coll == nil {
		return errors.New("events collection is not initialized")
	}

	event, err := r.GetEventByID(ctx, eventID)
	if err != nil {
		return err
	}
	if containsParticipant(event.CurrentParticipants, userID) {
		return nil
	}
	if event.MaxParticipants > 0 && len(event.CurrentParticipants) >= event.MaxParticipants {
		return ErrEventIsFull
	}

	res, err := coll.UpdateByID(ctx, eventID, bson.M{
		"$addToSet": bson.M{"current_participants": userID},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) RemoveEventParticipant(ctx context.Context, eventID string, userID int64) error {
	coll := r.eventsCollection()
	if coll == nil {
		return errors.New("events collection is not initialized")
	}

	res, err := coll.UpdateByID(ctx, eventID, bson.M{
		"$pull": bson.M{"current_participants": userID},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrCMSNotFound
	}
	return nil
}

func (r *MongoRepository) postsCollection() *mongo.Collection {
	if r.posts != nil {
		return r.posts
	}
	if r.db == nil {
		return nil
	}
	r.posts = r.db.Collection("posts")
	return r.posts
}

func (r *MongoRepository) eventsCollection() *mongo.Collection {
	if r.events != nil {
		return r.events
	}
	if r.db == nil {
		return nil
	}
	r.events = r.db.Collection("events")
	return r.events
}

func ensurePostDefaults(post *Post) {
	if post.ID == "" {
		post.ID = uuid.NewString()
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now().UTC()
	}
}

func ensureEventDefaults(event *Event) {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CurrentParticipants == nil {
		event.CurrentParticipants = make([]int64, 0)
	}
}

func containsParticipant(participants []int64, userID int64) bool {
	for _, id := range participants {
		if id == userID {
			return true
		}
	}
	return false
}

func removeParticipant(participants []int64, userID int64) []int64 {
	if len(participants) == 0 {
		return participants
	}
	out := make([]int64, 0, len(participants))
	for _, id := range participants {
		if id != userID {
			out = append(out, id)
		}
	}
	return out
}
