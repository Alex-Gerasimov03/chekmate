// Обработчики ввода: фотографии чеков, тексты трат и уточнение товаров.
package bot

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"

	// Пустые импорты регистрируют декодеры: без них image.Decode отвечает
	// "unknown format" на обычную фотографию из Telegram.
	_ "image/jpeg"
	_ "image/png"

	"io"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/Alex-Gerasimov03/chekmate/internal/qr"
	"github.com/Alex-Gerasimov03/chekmate/internal/service"
)

func (b *Bot) onPhoto(c tele.Context) error {
	photo := c.Message().Photo
	if photo == nil {
		return nil
	}
	if photo.FileSize > maxPhotoBytes {
		return c.Send("Снимок слишком большой. Пришлите фото поменьше или сожмите его.")
	}

	file, err := b.tb.File(&photo.File)
	if err != nil {
		b.log.Error("не удалось скачать фото", "err", err, "file_id", photo.FileID)
		return c.Send("Не удалось скачать фотографию, попробуйте ещё раз.")
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes))
	if err != nil {
		b.log.Error("не удалось прочитать фото", "err", err)
		return c.Send("Не удалось скачать фотографию, попробуйте ещё раз.")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		b.log.Error("не удалось разобрать изображение",
			"err", err, "байт", len(data), "начало", head(data))
		return c.Send("Не удалось прочитать изображение.")
	}
	b.log.Info("фото получено", "формат", format, "байт", len(data))

	raw, err := b.decodeQR(img)
	if err != nil {
		qrDecodeFailures.Inc()
		return c.Send("Не вижу QR-код. Снимите его крупнее и без бликов — " +
			"или пришлите трату текстом: <code>молоко 89</code>")
	}

	ctx, cancel := b.ctx()
	defer cancel()

	added, err := b.svc.AddFromQR(ctx, c.Chat().ID, c.Sender().ID, raw)
	switch {
	case errors.Is(err, qr.ErrNotReceipt):
		return c.Send("Это QR-код, но не кассовый чек.")
	case err != nil:
		handlerErrors.WithLabelValues("receipt").Inc()
		b.log.Error("не удалось добавить чек", "err", err)
		return c.Send("Не получилось сохранить чек. Попробуйте позже.")
	}
	track(added, service.SourceQR)

	if err := c.Send(formatAdded(added), addedKeyboard(added.ReceiptID)); err != nil {
		return err
	}
	return b.ask(c, added)
}

// decodeQR ограничивает число одновременных распознаваний: обработка идёт
// в три прохода и на большом снимке занимает заметное время.

func (b *Bot) decodeQR(img image.Image) (string, error) {
	select {
	case b.decode <- struct{}{}:
		defer func() { <-b.decode }()
	case <-time.After(requestTimeout):
		return "", qr.ErrNoQR
	}
	return qr.Decode(img)
}

func (b *Bot) onText(c tele.Context) error {
	if !b.addressed(c) {
		return nil
	}

	ctx, cancel := b.ctx()
	defer cancel()

	text := strings.TrimSpace(strings.ReplaceAll(c.Text(), "@"+b.username(), ""))
	if isMenuButton(text) {
		return nil
	}

	added, err := b.svc.AddManual(ctx, c.Chat().ID, c.Sender().ID, text)
	switch {
	case errors.Is(err, service.ErrNotEntry):
		return c.Send("Не понял. Напишите название и сумму: <code>молоко 89</code>")
	case err != nil:
		handlerErrors.WithLabelValues("manual").Inc()
		b.log.Error("не удалось добавить трату", "err", err)
		return c.Send("Не получилось сохранить трату. Попробуйте позже.")
	}
	track(added, service.SourceManual)

	if err := c.Send(formatAdded(added), addedKeyboard(added.ReceiptID)); err != nil {
		return err
	}
	return b.ask(c, added)
}

// ask задаёт вопросы о позициях, в которых матчер не уверен.

func (b *Bot) ask(c tele.Context, added service.Added) error {
	if len(added.Questions) == 0 {
		return nil
	}

	ctx, cancel := b.ctx()
	defer cancel()

	budgetID, err := b.budget(ctx, c)
	if err != nil {
		return b.fail(c, "уточнение", err)
	}

	for _, q := range added.Questions {
		id := b.pending.put(pendingQuestion{
			ReceiptID: q.ReceiptID,
			BudgetID:  budgetID,
			RawName:   q.RawName,
			ProductID: q.Suggestion.ProductID,
			Title:     q.Suggestion.Title,
		})
		if id == "" {
			continue
		}

		markup := &tele.ReplyMarkup{}
		markup.Inline(markup.Row(
			markup.Data("Да, это оно", btnYes.Unique, id),
			markup.Data("Нет, другой", btnNo.Unique, id),
		))

		if err := c.Send(formatQuestion(q), markup); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) onSameProduct(c tele.Context) error {
	q, ok := b.pending.take(strings.TrimSpace(c.Callback().Data))
	if !ok {
		return c.Edit("Вопрос устарел.")
	}

	ctx, cancel := b.ctx()
	defer cancel()

	if err := b.svc.ConfirmProduct(ctx, q.ReceiptID, q.RawName, q.ProductID); err != nil {
		b.log.Error("не удалось подтвердить товар", "err", err)
		return c.Edit("Не получилось сохранить ответ.")
	}
	return c.Edit("Запомнил: " + escape(q.RawName) + " — это " + escape(q.Title) + ".")
}

func (b *Bot) onOtherProduct(c tele.Context) error {
	q, ok := b.pending.take(strings.TrimSpace(c.Callback().Data))
	if !ok {
		return c.Edit("Вопрос устарел.")
	}

	ctx, cancel := b.ctx()
	defer cancel()

	if err := b.svc.RejectSuggestion(ctx, q.BudgetID, q.ReceiptID, q.RawName); err != nil {
		b.log.Error("не удалось завести товар", "err", err)
		return c.Edit("Не получилось сохранить ответ.")
	}
	return c.Edit("Завёл отдельный товар: " + escape(q.RawName) + ".")
}

func track(added service.Added, source string) {
	if added.Created {
		receiptsAdded.WithLabelValues(source).Inc()
		return
	}
	receiptsDuplicate.Inc()
}

// defaultInflationMonths — за сколько месяцев сравниваются цены по умолчанию.
// За более короткий срок изменение тонет в случайных колебаниях.

func (b *Bot) fail(c tele.Context, what string, err error) error {
	handlerErrors.WithLabelValues(what).Inc()
	b.log.Error("не удалось построить "+what, "err", err)
	return c.Send("Не получилось посчитать. Попробуйте позже.")
}

// onRule сохраняет личное правило категоризации и пересчитывает уже
// купленные товары.

func head(b []byte) string {
	if len(b) > 16 {
		b = b[:16]
	}
	return hex.EncodeToString(b)
}

// onTop показывает подорожавшие позиции без остального разбора индекса.
