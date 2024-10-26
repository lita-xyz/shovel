package sdk

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/indexsupply/shovel/shovel"
	"github.com/indexsupply/shovel/shovel/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pgp  *pgxpool.Pool
	mgr  *shovel.Manager
	conf *config.Root
}

func New(mgr *shovel.Manager, conf *config.Root, pgp *pgxpool.Pool) *Handler {
	h := &Handler{
		pgp:  pgp,
		mgr:  mgr,
		conf: conf,
	}
	return h
}

type OffRampRecordRequest struct {
	Address string `json:"address"`
}

type OffRampRecord struct {
	OffRampAmount  int    `json:"off_ramp_amount"`
	ConversionRate int    `json:"conversion_rate"`
	TxHash         string `json:"tx_hash"`
}

type OffRampRecordResponse struct {
	Records []OffRampRecord `json:"records"`
}

func (h *Handler) OffRampRecord(w http.ResponseWriter, r *http.Request) {
	var cc OffRampRecordRequest
	err := json.NewDecoder(r.Body).Decode(&cc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var query = fmt.Sprintf(`
		select 
			(off_ramp_amount, conversion_rate, tx_hash) 
		from 
			off_ramp_intents 
		where
			user = '%s'
		order by 
			block_num desc
	`,
		cc.Address)

	rows, err := h.pgp.Query(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	records, err := pgx.CollectRows(rows, pgx.RowToStructByName[OffRampRecord])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := OffRampRecordResponse{Records: records}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type OnRampRecordRequest struct {
	Address string `json:"address"`
}

type OnRampeRecord struct {
	ConversionRate  int    `json:"conversion_rate"`
	TxHash          string `json:"tx_hash"`
	Email           string `json:"email"`
	OffRampIntentId int    `json:"off_ramp_intent_id"`
}

type OnRampRecordResponse struct {
	Records []OnRampeRecord `json:"records"`
}

func (h *Handler) OnRampRecord(w http.ResponseWriter, r *http.Request) {
	var cc OnRampRecordRequest
	err := json.NewDecoder(r.Body).Decode(&cc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var query = fmt.Sprintf(`
		select 
			(conversion_rate, tx_hash, email, off_ramp_intent_id) 
		from 
			on_ramp_intents 
		where
			on_ramper_address = '%s'
		left join
			user_registrations on on_ramp_intents.off_ramper_address = user_registrations.wallet
		left join 
			off_ramp_intents on on_ramp_intents.off_ramp_intent_id = off_ramp_intents.off_ramp_intent_id
		order by 
			block_num desc
	`,
		cc.Address)

	rows, err := h.pgp.Query(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	records, err := pgx.CollectRows(rows, pgx.RowToStructByName[OnRampeRecord])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := OnRampRecordResponse{Records: records}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type AvailableOffRampIntents struct {
	OffRampIntentId int    `json:"off_ramp_intent_id"`
	WalletAddress   string `json:"wallet_address"`
	PayPalId        string `json:"paypal_id"`
	OffRampAmount   int    `json:"off_ramp_amount"`
	ConversionRate  int    `json:"conversion_rate"`
}

type AvailableOffRampIntentsResponse struct {
	Records []AvailableOffRampIntents `json:"records"`
}

func (h *Handler) AvailableOffRampsIntents(w http.ResponseWriter, r *http.Request) {

	query := `
		select 
			(off_ramp_intent_id, user, paypal_id, off_ramp_amount, conversion_rate)
		from 
			off_ramp_intents
		left join 
			book on book.off_ramp_intent_id = off_ramp_intents.off_ramp_intent_id
		left join 
			cancellation_intents on cancellation_intents.off_ramp_intent_id = off_ramp_intents.off_ramp_intent_id
		where 
			book.off_ramp_intent_id is null and cancellation_intents.off_ramp_intent_id is null
	`

	rows, err := h.pgp.Query(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	intents, err := pgx.CollectRows(rows, pgx.RowToStructByName[AvailableOffRampIntents])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := AvailableOffRampIntentsResponse{Records: intents}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

}

type EmailRequest struct {
	Address string `json:"address"`
}

type EmailResponse struct {
	Email string `json:"email"`
}

func (h *Handler) Email(w http.ResponseWriter, r *http.Request) {
	var wallet EmailRequest
	err := json.NewDecoder(r.Body).Decode(&wallet)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf(`
		select 
			email 
		from 
			user_registrations 
		where
			wallet = '%s'
	`, wallet.Address)

	row := h.pgp.QueryRow(r.Context(), query)
	emailResponse := EmailResponse{
		Email: "",
	}

	err = row.Scan(&emailResponse.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(emailResponse)

}

func (h *Handler) Book(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	off_ramp_intent_id := -1
	err := json.NewDecoder(r.Body).Decode(&off_ramp_intent_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if off_ramp_intent_id < 0 {
		http.Error(w, "invalid off_ramp_intent_id", http.StatusBadRequest)
		return
	}

	stmt := fmt.Sprintf(`
		insert into 
			book (off_ramp_intent_id, created_at) 
		values 
			(%d, now())
		where 
			not exists 
				(select off_ramp_intent_id from cancellation_intents where off_ramp_intent_id = %d)
	`, off_ramp_intent_id, off_ramp_intent_id)

	_, err = h.pgp.Exec(r.Context(), stmt)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
