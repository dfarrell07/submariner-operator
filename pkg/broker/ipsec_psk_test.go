package broker

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// FIXME: Use shared var for PSK byte array length everywhere
const testPSKLen = 48

var _ = Describe("ipsec_psk handling", func() {
	When("generateRandonPSK is called", func() {
		It("should return the amount of entropy requested", func() {
			psk, err := generateRandomPSK(testPSKLen)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(psk).To(HaveLen(testPSKLen))
		})
	})

	When("NewBrokerPSKSecret is called", func() {
		It("should return a secret with a psk data inside", func() {
			secret, err := NewBrokerPSKSecret(testPSKLen)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(secret.Name).To(Equal("submariner-ipsec-psk"))
			Expect(secret.Data).To(HaveKey("psk"))
			Expect(secret.Data["psk"]).To(HaveLen(testPSKLen))
		})
	})

	When("NewBrokerPSKByte is called", func() {
		It("should return psk as a string", func() {
			psk, err := NewBrokerPSKSecret(testPSKLen)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(psk).To(HaveLen(testPSKLen))
			// TODO: Assert type is []byte
		})
	})

	When("NewBrokerPSKString is called", func() {
		It("should return psk as a string", func() {
			psk, err := NewBrokerPSKSecret(testPSKLen)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(psk).To(HaveLen(testPSKLen))
			// TODO: Assert type is string
		})
	})
})
