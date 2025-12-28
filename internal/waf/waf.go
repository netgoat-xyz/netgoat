package waf

import (
	"database/sql"
	"net"
	"net/http"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/rs/zerolog/log"
)

type WAFContext struct {
	IP      string
	Method  string
	Path    string
	Headers map[string][]string
}

func Check(db *sql.DB, r *http.Request, debugLogs bool) (bool, string) {
	rows, err := db.Query("SELECT name, expression, action FROM waf_rules ORDER BY priority DESC")
	if err != nil {
		log.Error().Err(err).Msg("Failed to query WAF rules")
		return false, ""
	}
	defer rows.Close()

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}

	env := WAFContext{
		IP:      ip,
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header,
	}

	for rows.Next() {
		var name, expression, action string
		if err := rows.Scan(&name, &expression, &action); err != nil {
			continue
		}

		program, err := expr.Compile(expression, expr.Env(WAFContext{}))
		if err != nil {
			log.Error().Err(err).Str("rule", name).Msg("Invalid WAF rule expression")
			continue
		}

		output, err := expr.Run(program, env)
		if err != nil {
			log.Error().Err(err).Str("rule", name).Msg("Error running WAF rule")
			continue
		}

		matched, ok := output.(bool)
		if debugLogs {
			log.Debug().Str("rule", name).Str("expression", expression).Bool("matched", matched).Msg("WAF Rule Evaluation")
		}

		if ok && matched {
			if strings.ToUpper(action) == "BLOCK" {
				return true, name
			}
		}
	}
	return false, ""
}
