package main

import (
	"bytes"
	"fmt"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"net/http"
    "time"
)

// XXX: For actual encryption in app do not use armor for encoding

func getKeyByKeybaseUsername(username string) (openpgp.EntityList, error) {
	// Gets public key of recipient
	resp, err := http.Get("https://keybase.io/" + username + "/key.asc")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Reads key into list of keys
	entityList, err := openpgp.ReadArmoredKeyRing(resp.Body)
	if err != nil {
		return nil, err
	}
	return entityList, nil
}

func encrypt(message string, users []string) (string, error) {

	var entityList openpgp.EntityList

	for _, username := range users {
		eL, err := getKeyByKeybaseUsername(username)
		if err != nil {
			return "", err
		}
		entityList = append(entityList, eL[0])
	}

    // Print data from the public key
    for i := range entityList {
        for k := range entityList[i].Identities {
            fmt.Println("Name: " + entityList[i].Identities[k].UserId.Name)
            fmt.Println("Email: " + entityList[i].Identities[k].UserId.Email)
            fmt.Println("Comment: " + entityList[i].Identities[k].UserId.Comment)
            fmt.Println("Creation Time: " + entityList[i].Identities[k].SelfSignature.CreationTime.Format(time.UnixDate) + "\n")
        }
    }

	// New buffer where the result of the encripted msg will be
	buf := new(bytes.Buffer)

	// Create an armored template stream for msg
	w, err := armor.Encode(buf, "PGP MESSAGE", nil)
	if err != nil {
		return "", err
	}

	// Create an encryption stream
	plaintext, err := openpgp.Encrypt(w, entityList, nil, nil, nil)
	if err != nil {
		return "", err
	}

	// Write a byte array saying the message to encryption stream
	_, err = plaintext.Write([]byte(message))
	if err != nil {
		return "", err
	}

	// Close streams, finishing encryption and armor texts
	plaintext.Close()
	w.Close()

	return buf.String(), nil
}

func main() {
	str, err := encrypt("hello world", []string{"leijurv", "aj_n", "slaidan_lt"})
	if err != nil {
		panic(err)
	}
	fmt.Println(str)
}