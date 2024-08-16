package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/objectiveryan/irsal/internal/common"
)

type DbStorage struct {
	db  *sql.DB
	mut sync.Mutex
}

func NewInMemoryStorage() common.Storage {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	log.Printf("Cwd=%s", dir)
	s, err := NewSqliteStorage("file:" + uuid.NewString() + "?mode=memory&cache=shared")
	if err != nil {
		panic(err)
	}
	s.db.SetConnMaxLifetime(-1)
	return s
}

func NewSqliteStorage(filename string) (*DbStorage, error) {
	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	create table if not exists AnnotationMessages (
		annot_id text not null,
		refs text,
		hyp_group text not null,
		uri_id int64 not null,
		chat_id int64 not null,
		message_id int64 not null,
		unique (annot_id, chat_id),
		unique (chat_id, message_id)
	);
	create table if not exists Subscriptions (
		hyp_token text not null,
		hyp_group text not null,
		search_after int64 not null,
		chat_id int64 not null,
		unique (hyp_group, chat_id)
	);
	create table if not exists URIs (
		uri text not null,
		unique (uri)
	);
	`)

	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Println(closeErr)
		}
		return nil, err
	}

	return &DbStorage{db: db}, nil
}

func (s *DbStorage) Close() error {
	return s.db.Close()
}

func (s *DbStorage) MessageID(annotID string, chatID int64) (int, error) {
	s.Lock()
	defer s.Unlock()

	stmt, err := s.db.Prepare("select message_id from AnnotationMessages where annot_id = ? and chat_id = ?")
	if err != nil {
		return -1, err
	}
	defer stmt.Close()

	var messageID int
	err = stmt.QueryRow(annotID, chatID).Scan(&messageID)
	if err == sql.ErrNoRows {
		return -1, common.ErrNotFound
	} else if err != nil {
		return -1, err
	}
	return messageID, nil
}

func (s *DbStorage) uriID(uri string) (int64, error) {
	stmt, err := s.db.Prepare("insert into URIs values(?) on conflict do update set uri=uri returning rowid")
	if err != nil {
		return -1, err
	}
	defer stmt.Close()
	var rowid int64
	err = stmt.QueryRow(uri).Scan(&rowid)
	if err == sql.ErrNoRows {
		panic("upsert returned no rows")
	}
	return rowid, err
}

func (s *DbStorage) SetMessageID(annotID string, meta common.AnnotationMetadata, chatID int64, messageID int) error {
	uriID, err := s.uriID(meta.URI)
	if err != nil {
		return fmt.Errorf("failed to get ID for URI: %v", err)
	}
	stmt, err := s.db.Prepare("insert into AnnotationMessages values(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("DB.Prepare failed: %v", err)
	}
	defer stmt.Close()

	var refs_str interface{} // does this need to be sql.NullString?
	if len(meta.References) > 0 {
		refs_str = strings.Join(meta.References, "|")
	}
	_, err = stmt.Exec(annotID, refs_str, meta.HypGroup, uriID, chatID, messageID)
	return err
}

func (s *DbStorage) AnnotationID(chatID int64, messageID int) (string, common.AnnotationMetadata, error) {
	var noMeta common.AnnotationMetadata
	stmt, err := s.db.Prepare("select annot_id, refs, hyp_group, uri from AnnotationMessages am left join URIs u ON am.uri_id = u.rowid where chat_id = ? and message_id = ?")
	if err != nil {
		return "", noMeta, err
	}
	defer stmt.Close()
	var annotID string
	var refs_str sql.NullString
	var group string
	var uri string
	err = stmt.QueryRow(chatID, messageID).Scan(&annotID, &refs_str, &group, &uri)
	if err == sql.ErrNoRows {
		return "", noMeta, common.ErrNotFound
	} else if err != nil {
		return "", noMeta, err
	}
	var refs []string
	if refs_str.Valid {
		refs = strings.Split(refs_str.String, "|")
	}
	return annotID, common.AnnotationMetadata{refs, group, uri}, nil
}

func (s *DbStorage) AddSubscription(sub *common.Subscription) error {
	stmt, err := s.db.Prepare("insert into Subscriptions values(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	searchAfter := sub.SearchAfter.UnixMicro()
	result, err := stmt.Exec(sub.HypToken, sub.HypGroup, searchAfter, sub.ChatID)
	if err != nil {
		return err
	}
	nrows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if nrows != 1 {
		return fmt.Errorf("%d subs added; want 1", nrows)
	}
	return nil
}

func (s *DbStorage) Subscription(chatID int64, group string) (*common.Subscription, error) {
	stmt, err := s.db.Prepare("select hyp_token, search_after from Subscriptions where hyp_group = ? and chat_id = ?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var token string
	var searchAfter int64
	err = stmt.QueryRow(group, chatID).Scan(&token, &searchAfter)
	if err == sql.ErrNoRows {
		return nil, common.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &common.Subscription{token, group, time.UnixMicro(searchAfter), chatID}, nil
}

func (s *DbStorage) Subscriptions() ([]*common.Subscription, error) {
	rows, err := s.db.Query("select hyp_token, hyp_group, search_after, chat_id from Subscriptions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []*common.Subscription
	for rows.Next() {
		var sub common.Subscription
		var searchAfter int64
		err = rows.Scan(&sub.HypToken, &sub.HypGroup, &searchAfter, &sub.ChatID)
		if err != nil {
			return nil, err
		}
		sub.SearchAfter = time.UnixMicro(searchAfter)
		subs = append(subs, &sub)
	}
	return subs, nil
}

func (s *DbStorage) UpdateSubscription(sub *common.Subscription) error {
	stmt, err := s.db.Prepare("update Subscriptions set hyp_token = ?, search_after = ? where hyp_group = ? and chat_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	searchAfter := sub.SearchAfter.UnixMicro()
	result, err := stmt.Exec(sub.HypToken, searchAfter, sub.HypGroup, sub.ChatID)
	if err != nil {
		return err
	}
	nrows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 {
		return common.ErrNotFound
	}
	if nrows > 1 {
		return fmt.Errorf("%d subs updated; want 1", nrows)
	}
	return nil
}

func (s *DbStorage) Lock() {
	s.mut.Lock()
}

func (s *DbStorage) Unlock() {
	s.mut.Unlock()
}
