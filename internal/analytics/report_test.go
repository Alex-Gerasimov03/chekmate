package analytics

import (
	"math"
	"testing"
	"time"

	"github.com/Alex-Gerasimov03/chekmate/internal/domain"
)

func spend(id int64, title string, food bool, cur, prev string) CategorySpend {
	return CategorySpend{
		CategoryID: id, Title: title, IsFood: food,
		Current:  domain.MustParseMoney(cur),
		Previous: domain.MustParseMoney(prev),
	}
}

func testReport() Report {
	current, previous := MonthToDate(time.Date(2026, 9, 6, 12, 0, 0, 0, msk), msk)
	return BuildReport(current, previous, []CategorySpend{
		spend(1, "Молочное", true, "1340.00", "1196.43"),
		spend(2, "Мясо", true, "1980.00", "1394.37"),
		spend(3, "Химия", false, "640.00", "640.00"),
		spend(4, "Рыба", true, "0.00", "0.00"),
	}, 5)
}

func TestBuildReportOrdersBySpend(t *testing.T) {
	r := testReport()

	if len(r.Categories) != 3 {
		t.Fatalf("категорий %d, ожидалось 3 (пустая отброшена)", len(r.Categories))
	}
	if r.Categories[0].Title != "Мясо" {
		t.Errorf("первой должна идти самая крупная категория, получена %q", r.Categories[0].Title)
	}
	if want := domain.MustParseMoney("3960.00"); r.Total != want {
		t.Errorf("итого %s, ожидалось %s", r.Total, want)
	}
}

func TestReportChange(t *testing.T) {
	r := testReport()

	got, ok := r.Change()
	if !ok {
		t.Fatal("изменение должно считаться")
	}
	// 3960,00 против 3230,80 в прошлом периоде
	if math.Abs(got-0.2257) > 0.001 {
		t.Errorf("изменение %.4f, ожидалось около 0,2257", got)
	}
}

// Новая категория не должна показывать рост на бесконечность.
func TestCategoryChangeWithoutHistory(t *testing.T) {
	c := spend(1, "Аптека", false, "500.00", "0.00")

	if _, ok := c.Change(); ok {
		t.Error("без прошлых расходов изменение считать нельзя")
	}
}

func TestReportFoodShare(t *testing.T) {
	r := testReport()

	// (1340 + 1980) / 3960
	if got := r.FoodShare(); math.Abs(got-0.8383) > 0.001 {
		t.Errorf("доля еды %.4f, ожидалось около 0,8383", got)
	}
}

func TestReportAverageReceipt(t *testing.T) {
	r := testReport()

	if want := domain.MustParseMoney("792.00"); r.AverageReceipt() != want {
		t.Errorf("средний чек %s, ожидалось %s", r.AverageReceipt(), want)
	}
}

func TestReportEmpty(t *testing.T) {
	current, previous := MonthToDate(time.Now(), msk)
	r := BuildReport(current, previous, nil, 0)

	if r.Total != 0 || r.FoodShare() != 0 || r.AverageReceipt() != 0 {
		t.Errorf("пустой отчёт должен быть нулевым: %+v", r)
	}
	if _, ok := r.Change(); ok {
		t.Error("изменение на пустом отчёте считать нельзя")
	}
}
