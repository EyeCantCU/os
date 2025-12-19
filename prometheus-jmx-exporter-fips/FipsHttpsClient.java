import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.URL;
import java.security.Security;
import javax.net.ssl.HttpsURLConnection;
import javax.net.ssl.SSLContext;
import org.bouncycastle.jcajce.provider.BouncyCastleFipsProvider;

public class FipsHttpsClient {
    public static void main(String[] args) throws Exception {
        Security.addProvider(new BouncyCastleFipsProvider());

        // This will use your FIPS-configured trust store
        URL url = new URL("https://localhost:8889/metrics");
        HttpsURLConnection conn = (HttpsURLConnection) url.openConnection();

        try (BufferedReader reader = new BufferedReader(
                new InputStreamReader(conn.getInputStream()))) {
            String line;
            while ((line = reader.readLine()) != null) {
                if (line.contains("jvm_memory_used_bytes")) {
                    System.out.println(line);
                }
            }
        }
    }
}
