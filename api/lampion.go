package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/ezBadminton/ezBadmintonServer/tops"
	"github.com/pocketbase/pocketbase/core"
)

func BindLampionHooks(app core.App) {
	url := "/lampion"

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		group := rootGroup.Group(url)

		group.POST("", importLampion)
		group.POST("/upload", passThroughTournamentPlans)

		return e.Next()
	})
}

func importLampion(e *core.RequestEvent) error {
	if err := tops.ImportLampionTournament(); err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	return e.NoContent(http.StatusOK)
}

func passThroughTournamentPlans(e *core.RequestEvent) error {
	lampionSync := tops.GetLampionSync()
	if lampionSync == nil {
		return e.NoContent(http.StatusOK)
	}

	f, fileHeader, err := e.Request.FormFile("sheet")
	if err != nil {
		return e.BadRequestError("can not find file in body", err)
	}
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	defer f.Close()

	filename := fileHeader.Filename
	filenameNoExtension := strings.Split(filename, ".")[0]
	sheetKey := fmt.Sprintf("2025-%v", filenameNoExtension)

	buf := bytes.Buffer{}
	mpWriter := multipart.NewWriter(&buf)

	fileFieldHeader := make(textproto.MIMEHeader)
	fileFieldHeader.Set(
		"Content-Disposition",
		fmt.Sprintf(`form-data; name="sheet"; filename="%s"`, filename),
	)
	fileFieldHeader.Set("Content-Type", "application/pdf")
	fileField, err := mpWriter.CreatePart(fileFieldHeader)
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	_, err = io.Copy(fileField, f)
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	keyField, err := mpWriter.CreateFormField("sheetKey")
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	_, err = io.Copy(keyField, strings.NewReader(sheetKey))
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}

	mpWriter.Close()

	url := fmt.Sprintf("%v/tournament/sheet/upload", lampionSync.Url)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Authorization", lampionSync.ApiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return e.InternalServerError("something went wrong", err)
	}
	resBuf := strings.Builder{}
	io.Copy(&resBuf, res.Body)
	responseBody := resBuf.String()
	_ = responseBody

	return e.NoContent(http.StatusOK)
}
