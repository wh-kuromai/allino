## PROMPT

([PROMPT TEMPLATE](../README.md#ai-prompt-template-works-well-with-chatgpt) HERE)

Create simple QR code API which returns png image of QR code.

## ChatGPT RESULT

Here's a simple **QR code generator API** using the `allino` framework. This endpoint will accept a query parameter (`text`) and return a PNG image of the generated QR code.

---

### ✅ Dependencies

You need the popular Go QR code library:

```bash
go get github.com/skip2/go-qrcode
```

---

### 📦 Implementation

```go
import (
	"github.com/wh-kuromai/allino"
	"github.com/skip2/go-qrcode"
	"net/http"
)

type QRCodeInput struct {
	Text string `query:"text" validate:"required"`
}

var QRCodeAPI = allino.NewTypedHandler(
	&allino.HandlerOption{
		Path:        "/api/qrcode",
		Method:      "GET",
		ContentType: "image/png", // return raw image data
		Summary:     "Generate QR code",
		Description: "Returns a QR code image (PNG) for the given text.",
	},
	func(r *allino.Request, input *QRCodeInput) ([]byte, error) {
		png, err := qrcode.Encode(input.Text, qrcode.Medium, 256) // 256x256 PNG
		if err != nil {
			return nil, &allino.CodeError{
				StatusCode: http.StatusInternalServerError,
				Code:       "qrcode_generation_failed",
				Msg:        "Could not generate QR code",
			}
		}
		return png, nil // Returning []byte writes directly to the response
	},
)
```

---

### 🔍 Example Request

```http
GET /api/qrcode?text=hello%20world
Accept: image/png
```

This will return a PNG image of the QR code encoding `hello world`.

---

Let me know if you want to support SVG, dynamic size, or allow POST body input.
