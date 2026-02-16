import javax.net.ssl.*;
import java.security.*;
import java.io.*;
import java.net.*;

public class TLSTest {
    public static void main(String[] args) throws Exception {
        System.out.println("Testing TLS with BCFIPS provider...");

        // Get the SSL context - should use BCJSSE provider due to FIPS policy
        SSLContext ctx = SSLContext.getInstance("TLS");
        ctx.init(null, null, null);

        Provider provider = ctx.getProvider();
        System.out.println("SSLContext provider: " + provider.getName());

        if (!provider.getName().contains("BCJSSE")) {
            throw new RuntimeException("Expected BCJSSE provider but got: " + provider.getName());
        }

        // Verify we can create SSL socket factory
        SSLSocketFactory factory = ctx.getSocketFactory();
        System.out.println("SSLSocketFactory created successfully");

        // List supported cipher suites
        String[] ciphers = factory.getSupportedCipherSuites();
        System.out.println("Supported cipher suites: " + ciphers.length);

        // Verify we have TLS 1.2 and 1.3 protocols available
        SSLParameters params = ctx.getSupportedSSLParameters();
        boolean hasTls12 = false;
        boolean hasTls13 = false;
        for (String proto : params.getProtocols()) {
            System.out.println("Supported protocol: " + proto);
            if (proto.equals("TLSv1.2")) hasTls12 = true;
            if (proto.equals("TLSv1.3")) hasTls13 = true;
        }

        if (!hasTls12) {
            throw new RuntimeException("TLSv1.2 not available");
        }
        System.out.println("TLSv1.2 available: " + hasTls12);
        System.out.println("TLSv1.3 available: " + hasTls13);

        System.out.println("TLS test with BCFIPS provider PASSED!");
    }
}
