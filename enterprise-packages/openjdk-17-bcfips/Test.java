import javax.crypto.Cipher;
import javax.crypto.NoSuchPaddingException;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import java.security.*;
import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.stream.Stream;

public class Test {
    final static List<String> UNSUPPORTED_CIPHERS = Collections.unmodifiableList(Arrays.asList(
            // TLS v1.3
            "TLS_CHACHA20_POLY1305_SHA256",
            "TLS_AES_128_CCM_SHA256",

            // TLS v1.2
            "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", // OpenSSL: ECDHE-ECDSA-CHACHA20-POLY1305
            "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256", // OpenSSL: ECDHE-RSA-CHACHA20-POLY1305
            "TLS_ECDHE_ECDSA_WITH_AES_256_CCM", // OpenSSL: ECDHE-ECDSA-AES256-CCM
            "TLS_DHE_RSA_WITH_AES_256_CCM", // OpenSSL: DHE-RSA-AES256-CCM
            "TLS_ECDHE_ECDSA_WITH_AES_128_CCM", // OpenSSL: ECDHE-ECDSA-AES128-CCM
            "TLS_DHE_RSA_WITH_AES_128_CCM", // OpenSSL: DHE-RSA-AES128-CCM
            "TLS_DHE_RSA_WITH_CHACHA20_POLY1305_SHA256", // OpenSSL: DHE-RSA-CHACHA20-POLY1305
            "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA", // OpenSSL: ECDHE-PSK-AES128-CBC-SHA
            "TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256", // OpenSSL: ECDHE-PSK-AES128-CBC-SHA256
            "TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA", // OpenSSL: ECDHE-PSK-AES256-CBC-SHA
            "TLS_ECDHE_PSK_WITH_CHACHA20_POLY1305_SHA256", // OpenSSL: ECDHE-PSK-CHACHA20-POLY1305
            "TLS_PSK_WITH_AES_128_CBC_SHA", // OpenSSL: PSK-AES128-CBC-SHA
            "TLS_PSK_WITH_AES_128_CBC_SHA256", // OpenSSL: PSK-AES128-CBC-SHA256
            "TLS_PSK_WITH_AES_128_CCM", // OpenSSL: PSK-AES128-CCM
            "TLS_PSK_WITH_AES_128_GCM_SHA256", // OpenSSL: PSK-AES128-GCM-SHA256
            "TLS_PSK_WITH_AES_256_CBC_SHA", // OpenSSL: PSK-AES256-CBC-SHA
            "TLS_PSK_WITH_AES_256_CCM", // OpenSSL: PSK-AES256-CCM
            "TLS_PSK_WITH_AES_256_GCM_SHA384", // OpenSSL: PSK-AES256-GCM-SHA384
            "TLS_PSK_WITH_CHACHA20_POLY1305_SHA256", // OpenSSL: PSK-CHACHA20-POLY1305
            "TLS_RSA_PSK_WITH_AES_128_CBC_SHA", // OpenSSL: RSA-PSK-AES128-CBC-SHA
            "TLS_RSA_PSK_WITH_AES_128_CBC_SHA256", // OpenSSL: RSA-PSK-AES128-CBC-SHA256
            "TLS_RSA_PSK_WITH_AES_128_GCM_SHA256", // OpenSSL: RSA-PSK-AES128-GCM-SHA256
            "TLS_RSA_PSK_WITH_AES_256_CBC_SHA", // OpenSSL: RSA-PSK-AES256-CBC-SHA
            "TLS_RSA_PSK_WITH_AES_256_GCM_SHA384", // OpenSSL: RSA-PSK-AES256-GCM-SHA384
            "TLS_RSA_PSK_WITH_CHACHA20_POLY1305_SHA256" // OpenSSL: RSA-PSK-CHACHA20-POLY1305
    ));

    private static void testDigestCiphers() {
        try {
            MessageDigest digest = MessageDigest.getInstance("MD5");
            System.out.println(digest.getAlgorithm());
            System.out.println(digest.getProvider());

            System.out.println("MD5 is always available");
            System.out.println(
                    "* see table 7 of https://downloads.bouncycastle.org/fips-java/BC-FJA-SecurityPolicy-1.0.2.pdf");
            System.out.println("* see https://github.com/bcgit/bc-java/issues/1282");
        } catch (final NoSuchAlgorithmException e) {
            // this is unreachable code. MD5 is always available.
        }

        try {
            Cipher cipher = Cipher.getInstance("Blowfish/CBC/PKCS5Padding");
            System.out.println(cipher.getAlgorithm());
            System.out.println(cipher.getProvider());

            System.err.println("Blowfish should not be available. Validation failed.");
            System.exit(1);
        } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
            // expected
            System.out.println("Blowfish is not available. Validation passed.");
            System.out.println("Details: ");
            e.printStackTrace();
        }

        try {
            Cipher.getInstance("AES/CBC/NoPadding");
            Cipher.getInstance("AES/CBC/PKCS5Padding");
            System.out.println("AES cipher is available. Validation passed.");
        } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
            System.err.println("AES is not available. Validation failed.");
            e.printStackTrace();
            System.exit(1);
        }

        try {
            Cipher.getInstance("AES/EAX/NoPadding");
            System.err.println("AES cipher is available in EAX mode. Validation failed.");
            System.exit(1);
        } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
            // AES/EAX is not available
            System.out.println("AES/EAX is not available. Validation passed.");
            System.out.println("Details: ");
            e.printStackTrace();
        }

        try {
            Cipher.getInstance("ChaCha20");
            System.err.println("ChaCha20 cipher is available. Validation failed.");
            System.exit(1);
        } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
            // ChaCha20 is not available
            System.out.println("ChaCha20 is not available. Validation passed.");
            System.out.println("Details: ");
            e.printStackTrace();
        }
    }

    private static void testSSLCiphers() {
        System.out.println(Arrays.asList(Security.getProviders()));

        SSLContext sslContext = null;

        try {
            final KeyStore trustStore = KeyStore.getInstance(KeyStore.getDefaultType());
            trustStore.load(null, null);

            final TrustManagerFactory factory = TrustManagerFactory.getInstance("PKIX");
            factory.init(trustStore);

            sslContext = SSLContext.getInstance("TLS");
            sslContext.init(null, factory.getTrustManagers(), SecureRandom.getInstance("DEFAULT"));

            if (!"BCJSSE".equals(sslContext.getProvider().getName())) {
                System.err.println("failed to verify provider BCJSSE");
                System.exit(1);
            }
        } catch (final Exception e) {
            System.err.println("failed to get default SSL context: ");
            e.printStackTrace();
            System.exit(1);
        }

        Provider sslProvider = sslContext.getProvider();
        System.out.println(sslProvider);
        System.out.println(Stream.of(sslContext.getSupportedSSLParameters().getCipherSuites())
                .reduce("\t", (accumulated, val) -> accumulated + val + ",\n\t"));

        // check unsupported ciphers first
        for (final String cipher : sslContext.getSupportedSSLParameters().getCipherSuites()) {
            try {
                if (UNSUPPORTED_CIPHERS.contains(cipher)) {
                    Cipher.getInstance(cipher, sslProvider);
                    System.err.println("cipher " + cipher + " is prohibited, but remains available");
                    System.exit(1);
                }
            } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
                // expected, continue
            }
        }
    }

    public static void main(String[] args) throws Exception {
        if (!org.bouncycastle.crypto.fips.FipsStatus.isReady()) {
            System.err.println("fips status is not ready");
            System.exit(1);
        }

        System.out.println("Available providers: " + Arrays.asList(Security.getProviders()));

        testDigestCiphers();
        testSSLCiphers();
    }
}
