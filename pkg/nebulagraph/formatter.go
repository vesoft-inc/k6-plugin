package nebulagraph

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vesoft-inc/nebula-http-gateway/ccore/nebula/types"
)

func ValueToString(value types.Value) string {
	if value.IsSetNVal() {
		return value.GetNVal().String()
	} else if value.IsSetBVal() {
		return fmt.Sprintf("%t", value.GetBVal())
	} else if value.IsSetIVal() {
		return fmt.Sprintf("%d", value.GetIVal())
	} else if value.IsSetFVal() {
		fStr := strconv.FormatFloat(value.GetFVal(), 'g', -1, 64)
		if !strings.Contains(fStr, ".") {
			fStr = fStr + ".0"
		}
		return fStr
	} else if value.IsSetSVal() {
		return `` + string(value.GetSVal()) + ``
	} else if value.IsSetDVal() { // Date yyyy-mm-dd
		date := value.GetDVal()
		return fmt.Sprintf("%04d-%02d-%02d",
			date.GetYear(),
			date.GetMonth(),
			date.GetDay())
	} else if value.IsSetTVal() { // Time HH:MM:SS.MSMSMS
		rawTime := value.GetTVal()
		return fmt.Sprintf("%02d:%02d:%02d.%06d",
			rawTime.GetHour(),
			rawTime.GetMinute(),
			rawTime.GetSec(),
			rawTime.GetMicrosec())
	} else if value.IsSetDtVal() { // DateTime yyyy-mm-ddTHH:MM:SS.MSMSMS
		rawDateTime := value.GetDtVal()
		return fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d.%06d",
			rawDateTime.GetYear(),
			rawDateTime.GetMonth(),
			rawDateTime.GetDay(),
			rawDateTime.GetHour(),
			rawDateTime.GetMinute(),
			rawDateTime.GetSec(),
			rawDateTime.GetMicrosec())
	} else {
		return "not support"
	}
}
