package miscutils

import (
	"fmt"
	"reflect"
	"strings"
)

/*
// MapKeysToList returns the keys of a map[string]string as a list.
func MapKeysToList(theMap map[string]string) []string {
	keys := make([]string, 0, len(theMap))

	for k := range theMap {
		keys = append(keys, k)
	}

	return keys
}

// GetAddrPort get Address and Port from: 'protocol://address:port/...'
func GetAddrPort(URL, defPort string) (string, string, string) {
	if strings.Contains(URL, "://") {
		// strip protocol
		URL = URL[strings.Index(URL, ":")+2:]
	}
	if strings.Contains(URL, "/") {
		URL = URL[:strings.Index(URL, "/")]
	}
	split := strings.Index(URL, ":")
	theAddr := URL
	thePort := defPort
	theAddrPort := fmt.Sprintf("%s:%s", URL, defPort)
	if split > 0 {
		theAddr = URL[:split]
		thePort = URL[split+1:]
		theAddrPort = fmt.Sprintf("%s:%s", theAddr, thePort)
	}
	return theAddrPort, theAddr, thePort
}


// GetFilesList is used for debugging purposes to list the files in the specified directory subtree
func GetFilesList(dir string) ([]string, error) {
	fileList := []string{}
	files, err := io.ReadDir(dir)
	SecLog.Debugf("directory: %s", dir)
	if err != nil {
		errMsg := fmt.Sprintf("failed to process directory: %s", dir)
		return nil, RetErr(errMsg, err)
	}
	for _, file := range files {
		subDir := dir
		if file.IsDir() {
			subDir = dir + "/" + file.Name()
			SecLog.Debugf("sub directory: %s", subDir)
			subFileList, err := GetFilesList(subDir)
			if err != nil {
				errMsg := fmt.Sprintf("failed to process subdirectory: %s", subDir)
				return nil, RetErr(errMsg, err)
			}
			fileList = append(fileList, subFileList...)
			continue
		}
		fileList = append(fileList, subDir+"/"+file.Name())
		SecLog.Debugf("file path: %s", subDir+"/"+file.Name())

	}
	return fileList, nil
}

// OpenOrCreate opens or creates a file returning file descriptor
func OpenOrCreate(fileName string) (*os.File, error) {
	lastSlash := strings.LastIndex(fileName, "/")
	if lastSlash < 0 {
		lastSlash = 0
	}
	dir := fileName[:lastSlash]
	if len(dir) > 0 {
		err := os.MkdirAll(dir, os.FileMode(0755))
		if err != nil {
			return nil, RetErr("failed to create directory", err)
		}
	}
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_APPEND, os.ModeAppend)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(fileName)
			if err != nil {
				return nil, RetErr(fmt.Sprintf("failed to create file: %s", fileName), err)
			}
		} else {
			return nil, RetErr(fmt.Sprintf("failed to open file: %s", fileName), err)
		}
	}
	return file, nil
}

// StringToFile writes a string to a file
func StringToFile(fileName, data string, truncate bool) error {
	defer TraceExit(Trace())

	file, err := OpenOrCreate(fileName)
	if err != nil {
		return RetErr(fmt.Sprintf("failed to open or create file: %s", fileName), err)
	}
	defer file.Close()

	if truncate {
		err = file.Truncate(0)
		if err != nil {
			return NewError("failed to truncate file %s, %s", fileName, err)
		}
		_, err = file.Seek(0, 0)
		if err != nil {
			return NewError("failed to reset write position to start of file %s, %s", fileName, err)
		}
	}

	_, err = file.WriteString(data)
	if err != nil {
		return RetErr(fmt.Sprintf("failed to write data to file: %s", fileName), err)
	}
	return nil
}

// compareStringArray string arrays sorting them to verify that they contain
// equivalent information
func compareStringArray(one, two []string) bool {
	if len(one) != len(two) {
		return false
	}
	sort.Strings(one)
	sort.Strings(two)
	for index, field := range one {
		if field != two[index] {
			return false
		}
	}
	return true
}

/* func decryptWithKey(ciphertext []byte, key *[32]byte) (plaintext []byte, err error) {
	defer TraceExit(Trace())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, RetErr("NewCipher failed", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, RetErr("NewGCM failed", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, NewError("malformed ciphertext")
	}

	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
}

func encryptWithKey(plaintext []byte, key *[32]byte) (ciphertext []byte, err error) {
	defer TraceExit(Trace())

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, RetErr("NewCipher failed", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, RetErr("NewGCM failed", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, RetErr("unable to obtain random data", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
} */

// Max returns the maximum of two integers.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FindInSlice determines if a string is in a slice of strings.
func FindInSlice(value string, slice []string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// TypeExaminer returns a string containing details of an interface type, useful for debugging and understanding data.
func TypeExaminer(t reflect.Type, depth int, data interface{}) string {
	details := fmt.Sprintf("%sType: %-10s kind: %-8s Package: %s\n", strings.Repeat("\t", depth), t.Name(), t.Kind(), t.PkgPath())
	switch t.Kind() { //nolint: exhaustive
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
		details += fmt.Sprintf("%sContained type...\n", strings.Repeat("\t", depth+1))
		details += TypeExaminer(t.Elem(), depth+1, data)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			details += fmt.Sprintf("%sField: %d: name: %-10s type: %-8s kind: %-12s anon: %-6t package: %s\n",
				strings.Repeat("\t", depth+1), i+1, f.Name, f.Type.Name(), f.Type.Kind().String(), f.Anonymous, f.PkgPath)
			if f.Tag != "" {
				details += fmt.Sprintf("%sTags: %s\n", strings.Repeat("\t", depth+2), f.Tag) //nolint: mnd
			}
			if f.Type.Kind() == reflect.Struct {
				details += TypeExaminer(f.Type, depth+2, data) //nolint: mnd
			}
		}
	}
	return details
}
