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
