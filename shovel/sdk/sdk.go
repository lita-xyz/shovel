package sdk

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

type RecordRequest struct {
	Address  string `json:"address"`
	Limits   uint   `json:"limits,omitempty"`
	Page     uint   `json:"page,omitempty"`
	Status   string `json:"status,omitempty"`
	RampType string `json:"ramptype,omitempty"`
}

type Record struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Value    string `json:"value"`
	Status   string `json:"status"`
	RampType string `json:"type"`
	TxHash   string `json:"tx_hash"`
}

type OffRampIntents struct {
	ID             int    `json:"id"`
	PayPalId       string `json:"paypal_id"`
	Amount         string `json:"amount"`
	ConversionRate string `json:"conversion_rate"`
}

func New(mgr *shovel.Manager, conf *config.Root, pgp *pgxpool.Pool) *Handler {
	h := &Handler{
		pgp:  pgp,
		mgr:  mgr,
		conf: conf,
	}
	return h
}

func (h *Handler) Records(w http.ResponseWriter, r *http.Request) {
	var cc RecordRequest
	err := json.NewDecoder(r.Body).Decode(&cc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var status strings.Builder
	var ramptype strings.Builder
	var limit uint
	var offset uint
	if cc.Status != "" {
		status.WriteString(fmt.Sprintf("AND status = '%t'", cc.Status == "success"))
	}
	if cc.RampType != "" {
		ramptype.WriteString(fmt.Sprintf("AND type = '%t'", cc.RampType == "on"))
	}

	if cc.Limits == 0 {
		limit = 20
	} else {
		limit = cc.Limits
	}

	offset = (cc.Page - 1) * limit

	var query = fmt.Sprintf(`
		SELECT (from, to, value, status, type, tx_hash) 
		FROM shovel.ramping 
		WHERE (from = %s OR to = %s)
		%s %s
		ORDER BY block_time DESC
		LIMIT %d offset %d
	`,
		cc.Address, cc.Address, status.String(),
		ramptype.String(), limit, offset)

	rows, err := h.pgp.Query(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	records, err := pgx.CollectRows(rows, pgx.RowToStructByName[Record])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(records)
}

func (h *Handler) AvailableOffRamps(w http.ResponseWriter, r *http.Request) {

	query := `
		select 
			off_ramp_intent_id, 
			paypal_id, 
			off_ramp_amount, 
			conversion_rate
		from 
			off_ramp_intents
		left join book on book.off_ramp_intent_id = off_ramp_intents.off_ramp_intent_id
		left join withdraw on withdraw.off_ramp_intent_id = off_ramp_intents.off_ramp_intent_id
		where 
			book.off_ramp_intent_id is null and withdraw.off_ramp_intent_id is null
	`

	rows, err := h.pgp.Query(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	intents, err := pgx.CollectRows(rows, pgx.RowToStructByName[OffRampIntents])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(intents)

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
		where not exists (select off_ramp_intent_id from withdraw where off_ramp_intent_id = %d)
	`, off_ramp_intent_id, off_ramp_intent_id)

	_, err = h.pgp.Exec(r.Context(), stmt)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
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
			withdraw (off_ramp_intent_id, created_at) 
		values 
			(%d, now())
		where not exists (select off_ramp_intent_id from book where off_ramp_intent_id = %d)
	`, off_ramp_intent_id, off_ramp_intent_id)

	_, err = h.pgp.Exec(r.Context(), stmt)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
