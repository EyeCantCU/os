import javax.crypto.Cipher;
import javax.crypto.NoSuchPaddingException;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import java.security.*;
import java.util.Arrays;
import java.util.Map;
import java.util.Collections;
import java.util.List;
import java.util.stream.Stream;

public class Test {
    final static List<String> UNSUPPORTED_TLS_CIPHERS = Collections.unmodifiableList(Arrays.asList(
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

    
    final static Map<String,String> UNSUPPORTED_CIPHER_MODES = Map.ofEntries(
        Map.entry("Blowfish", "Blowfish/CBC/PKCS5Padding"),
        Map.entry("AES in EAX Mode", "AES/EAX/NoPadding"),
        Map.entry("ChaCha20", "ChaCha20"),
        Map.entry("Arc4", "Arc4"),
        Map.entry("Camellia", "Camellia/CCM/PKCS5Padding"),
        Map.entry("CAST5", "CAST5/CFB8/PKCS5Padding"),
        Map.entry("DES", "DES/CBC/NoPadding"),
        Map.entry("GOST28147", "GOST28147/CFB64/NoPadding"),
        Map.entry("IDEA", "IDEA/OFB/NoPadding"),
        Map.entry("RC2", "RC2/CTR/NoPadding"),
        Map.entry("SEED", "SEED/GCM/NoPadding"),
        Map.entry("Serpent", "Serpent/CCM/NoPadding"),
        Map.entry("SHACAL-2", "SHACAL-2/CTR/PKCS5Padding"),
        Map.entry("TripleDES in EAX", "DESede/EAX/NoPadding"),
        Map.entry("Twofish", "Twofish/OFB/NoPadding")
    );

    final static Map<String,String> SUPPORTED_CIPHER_MODES = Map.of(
        // According to BC:
        // FIPS is largely ambivalent towards padding mechanisms, so all modes are available in both approved-mode and general operation
        // We add a couple modes / paddings just to be sure
        "AES in CBC, NoPadding", "AES/CBC/NoPadding",
        "AES in ECB, NoPadding", "AES/ECB/NoPadding",
        "AES in CBC, PKCS5Padding", "AES/CBC/PKCS5Padding",
        "TripleDES in ECB, NOPadding", "DESede/ECB/NoPadding",
        "TripleDES in CBC, NoPadding", "DESede/CBC/NoPadding",
        "TripleDES in ECB, PKCS5Padding", "DESede/ECB/PKCS5Padding"
    );

    final static List<String> SUPPORTED_DIGESTS = Collections.unmodifiableList(Arrays.asList(
        "SHA-1",
        "SHA-224",
        "SHA-256",
        "SHA-384",
        "SHA-512",
        "SHA-512(224)",
        "SHA-512(256)",
        "SHA3-224",
        "SHA3-256",
        "SHA3-384",
        "SHA3-512",
        "SHAKE128",
        "SHAKE256"
    ));

    final static List<String> UNSUPPORTED_DIGESTS = Collections.unmodifiableList(Arrays.asList(
        "GOST3411",
        "GOST3411-2012-256",
        "GOST3411-2012-512",
        "RIPEMD128",
        "RIPEMD160",
        "RIPEMD256",
        "RIPEMD320",
        "Tiger",
        "Whirlpool",
        "MD5" // MD5 is not allowed when provider is BCFIPS
    ));

    private static void testMessageDigestsAndCiphers() {

        try {
            MessageDigest digest = MessageDigest.getInstance("MD5");
            System.out.println(digest.getAlgorithm());
            System.out.println(digest.getProvider());
            System.err.println("MD5 is allowed when provider is not specified (aka SUN). Validation Passed.");
        } catch (NoSuchAlgorithmException e) {
            // expected
            System.out.println("MD5 is not available. Validation failed.");
            System.out.println("Details: ");
            e.printStackTrace();
            System.exit(1);
        }

        for (String entry : SUPPORTED_DIGESTS) {
            try {
                MessageDigest digest = MessageDigest.getInstance(entry, "BCFIPS");
                System.out.println(digest.getAlgorithm());
                System.out.println(digest.getProvider());

                // expected
                System.out.println(entry + " is available. Validation passed.");
            } catch (NoSuchAlgorithmException | NoSuchProviderException e) {
                System.err.println(entry + " is not available. Validation failed. See page 37 of https://downloads.bouncycastle.org/fips-java/docs/BC-FJA-UserGuide-2.0.0.pdf");
                System.out.println("Details: ");
                e.printStackTrace();
                System.exit(1);
            }
        }

        for (String entry : UNSUPPORTED_DIGESTS) {
            try {
                MessageDigest digest = MessageDigest.getInstance(entry, "BCFIPS");
                System.out.println(digest.getAlgorithm());
                System.out.println(digest.getProvider());

                System.err.println(entry + " is available. Validation failed. See page 37 of https://downloads.bouncycastle.org/fips-java/docs/BC-FJA-UserGuide-2.0.0.pdf");
                System.exit(1);
            } catch (NoSuchAlgorithmException | NoSuchProviderException e) {
                // expected
                System.out.println(entry + " is not available. Validation passed.");
                System.out.println("Details: ");
                e.printStackTrace();
            }
        }

        for (Map.Entry<String, String> entry : UNSUPPORTED_CIPHER_MODES.entrySet()) {
            try {
                Cipher cipher = Cipher.getInstance(entry.getValue());
                System.out.println(cipher.getAlgorithm());
                System.out.println(cipher.getProvider());

                System.err.println(entry.getKey() + " is available. Validation failed.");
                System.exit(1);
            } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
                // expected
                System.out.println(entry.getKey() + " is not available. Validation passed.");
                System.out.println("Details: ");
                e.printStackTrace();
            }
        }

        for (Map.Entry<String, String> entry : SUPPORTED_CIPHER_MODES.entrySet()) {
            try {
                Cipher cipher = Cipher.getInstance(entry.getValue());
                System.out.println(cipher.getAlgorithm());
                System.out.println(cipher.getProvider());

                System.out.println(entry.getKey() + " is available. Validation passed.");
            } catch (NoSuchAlgorithmException | NoSuchPaddingException e) {
                System.err.println(entry.getKey() + " is not available. Validation failed.");
                e.printStackTrace();
                System.exit(1);
            }
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
                if (UNSUPPORTED_TLS_CIPHERS.contains(cipher)) {
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

        testMessageDigestsAndCiphers();
        testSSLCiphers();
    }
}
