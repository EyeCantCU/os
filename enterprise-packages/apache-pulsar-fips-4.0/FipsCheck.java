public class FipsCheck {
    public static void main(String[] args) throws Exception {
        // Default keystore type must be BCFKS
        String t = java.security.KeyStore.getDefaultType();
        if (!"BCFKS".equalsIgnoreCase(t)) { System.err.println("FAIL: default keystore type is " + t); System.exit(1); }
        System.out.println("PASS: getDefaultType = " + t);

        // Weak RSA key must be rejected (FIPS minimum: 2048-bit)
        try {
            java.security.KeyPairGenerator kpg = java.security.KeyPairGenerator.getInstance("RSA");
            kpg.initialize(1024);
            kpg.generateKeyPair();
            System.err.println("FAIL: 1024-bit RSA accepted"); System.exit(1);
        } catch (Error | Exception e) {
            System.out.println("PASS: 1024-bit RSA rejected (" + e.getClass().getSimpleName() + ")");
        }
    }
}
