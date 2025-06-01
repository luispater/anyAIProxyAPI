package method

import log "github.com/sirupsen/logrus"

func (m *Method) StringEqual(str1, str2 string) bool {
	log.Debugf("StringEqual result is %v, data:%s : %s", str1 == str2, str1, str2)
	return str1 == str2
}
