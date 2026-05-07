import * as forge from 'node-forge'

export function rsaEncrypt(data: string, publicKey: string) {
  const pk = forge.pki.publicKeyFromPem(publicKey)
  const encryptedBytes = pk.encrypt(data, 'RSA-OAEP', {
    md: forge.md.sha512.create(),
  })
  return forge.util.encode64(encryptedBytes)
}

export function rsaDecrypt(data: string, privateKey: string) {
  const pk = forge.pki.privateKeyFromPem(privateKey)
  return pk.decrypt(forge.util.decode64(data), 'RSA-OAEP', {
    md: forge.md.sha512.create(),
  })
}
