package qr

// QR codes compute error correction with Reed-Solomon codes over GF(256),
// with the field polynomial x^8 + x^4 + x^3 + x^2 + 1 and generator roots
// α^0 … α^(n-1), α = 2.

const fieldPolynomial = 0x11D

// gfMultiply multiplies two field elements (Russian peasant method, reduced
// by the field polynomial).
func gfMultiply(x, y byte) byte {
	z := 0
	for i := 7; i >= 0; i-- {
		z = z<<1 ^ (z>>7)*fieldPolynomial
		z ^= int(y>>i&1) * int(x)
	}
	return byte(z)
}

// rsDivisor returns the generator polynomial of degree n, without its
// leading 1, highest power first.
func rsDivisor(n int) []byte {
	result := make([]byte, n)
	result[n-1] = 1
	root := byte(1)
	for range n {
		for j := range n {
			result[j] = gfMultiply(result[j], root)
			if j+1 < n {
				result[j] ^= result[j+1]
			}
		}
		root = gfMultiply(root, 2)
	}
	return result
}

// rsRemainder returns the error correction codewords of data: the
// remainder of data × x^n divided by the generator.
func rsRemainder(data, divisor []byte) []byte {
	result := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i, d := range divisor {
			result[i] ^= gfMultiply(d, factor)
		}
	}
	return result
}
