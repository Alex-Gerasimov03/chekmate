package bot

import "github.com/prometheus/client_golang/prometheus"

var (
	receiptsAdded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chekmate_receipts_added_total",
		Help: "Сохранённые траты по источнику.",
	}, []string{"source"})

	receiptsDuplicate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chekmate_receipts_duplicate_total",
		Help: "Повторно присланные чеки.",
	})

	handlerErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chekmate_handler_errors_total",
		Help: "Ошибки обработчиков по типу операции.",
	}, []string{"kind"})

	qrDecodeFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chekmate_qr_decode_failures_total",
		Help: "Снимки, на которых не нашёлся QR-код.",
	})
)

func init() {
	prometheus.MustRegister(receiptsAdded, receiptsDuplicate, handlerErrors, qrDecodeFailures)
}
