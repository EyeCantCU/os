import java.io.FileOutputStream;
import java.security.*;
import java.security.cert.Certificate;
import java.security.cert.X509Certificate;
import java.math.BigInteger;
import java.util.Date;
import org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider;
import org.bouncycastle.asn1.x500.X500Name;
import org.bouncycastle.cert.X509v3CertificateBuilder;
import org.bouncycastle.cert.jcajce.JcaX509CertificateConverter;
import org.bouncycastle.cert.jcajce.JcaX509v3CertificateBuilder;
import org.bouncycastle.operator.ContentSigner;
import org.bouncycastle.operator.jcajce.JcaContentSignerBuilder;

public class GenerateKeystore {
    public static void main(String[] args) throws Exception {
        Security.addProvider(new BouncyCastleFipsProvider());

        KeyPairGenerator keyGen = KeyPairGenerator.getInstance("RSA", "BCFIPS");
        keyGen.initialize(2048);
        KeyPair keyPair = keyGen.generateKeyPair();

        X500Name subject = new X500Name("CN=localhost");
        BigInteger serial = BigInteger.valueOf(System.currentTimeMillis());
        Date notBefore = new Date();
        Date notAfter = new Date(notBefore.getTime() + 365L * 24 * 60 * 60 * 1000);

        X509v3CertificateBuilder certBuilder = new JcaX509v3CertificateBuilder(
            subject, serial, notBefore, notAfter, subject, keyPair.getPublic());

        ContentSigner signer = new JcaContentSignerBuilder("SHA256withRSA")
            .setProvider("BCFIPS").build(keyPair.getPrivate());

        X509Certificate cert = new JcaX509CertificateConverter()
            .setProvider("BCFIPS").getCertificate(certBuilder.build(signer));

        KeyStore ks = KeyStore.getInstance("BCFKS", "BCFIPS");
        ks.load(null, null);
        ks.setKeyEntry("jmx-exporter", keyPair.getPrivate(),
          "changeit".toCharArray(), new Certificate[]{cert});

        try (FileOutputStream fos = new FileOutputStream("/tmp/jmx_exporter.bcfks")) {
          ks.store(fos, "changeit".toCharArray());
        }

        System.out.println("BCFKS keystore created successfully with FIPS-approved crypto");
    }
}
