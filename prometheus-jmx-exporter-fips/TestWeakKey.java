import java.security.*;
import org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider;

public class TestWeakKey {
    public static void main(String[] args) {
        Security.addProvider(new BouncyCastleFipsProvider());

        System.out.println("Attempting to create 512-bit RSA key with BCFIPS...");
        System.out.println("This should FAIL as 512-bit is below FIPS minimum (2048-bit)");

        try {
            KeyPairGenerator keyGen = KeyPairGenerator.getInstance("RSA", "BCFIPS");
            keyGen.initialize(512);
            KeyPair keyPair = keyGen.generateKeyPair();

            System.out.println("ERROR: BCFIPS allowed 512-bit RSA key generation!");
            System.out.println("FIPS mode is NOT properly enforcing key strength requirements!");
            System.exit(1);
        } catch (Error e) {
            // BCFIPS throws FipsUnapprovedOperationError for unapproved operations
            System.out.println("SUCCESS: BCFIPS correctly rejected 512-bit RSA key");
            System.out.println("Rejection type: " + e.getClass().getSimpleName());
            System.out.println("Rejection reason: " + e.getMessage());
            System.exit(0);
        } catch (Exception e) {
            System.out.println("Unexpected exception: " + e.getClass().getName());
            System.out.println("Message: " + e.getMessage());
            System.exit(1);
        }
    }
}
