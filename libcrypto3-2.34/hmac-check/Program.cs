using System;
using System.Security.Cryptography;
using System.Text;

class Program
{
    static int Main(string[] args)
    {
        // Sample key and message
        string keyString = "my-secret-key";
        string message = "Hello, HMAC World!";

        byte[] key = Encoding.UTF8.GetBytes(keyString);
        byte[] messageBytes = Encoding.UTF8.GetBytes(message);

        // Calculate HMAC-SHA256
        Console.WriteLine("=== HMAC-SHA256 Calculation ===");
        Console.WriteLine($"Key: {keyString}");
        Console.WriteLine($"Message: {message}");

        using (var hmacSha256 = new HMACSHA256(key))
        {
            byte[] hash = hmacSha256.ComputeHash(messageBytes);
            string hashHex = BitConverter.ToString(hash).Replace("-", "").ToLower();
            Console.WriteLine($"HMAC-SHA256: {hashHex}");
        }

        Console.WriteLine();

        // Attempt HMAC-MD5 calculation
        Console.WriteLine("=== HMAC-MD5 Attempt ===");
        bool hmacMd5Succeeded = false;

        try
        {
            using (var hmacMd5 = new HMACMD5(key))
            {
                byte[] hash = hmacMd5.ComputeHash(messageBytes);
                string hashHex = BitConverter.ToString(hash).Replace("-", "").ToLower();
                Console.WriteLine($"HMAC-MD5: {hashHex}");
                hmacMd5Succeeded = true;
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"HMAC-MD5 failed (expected): {ex.GetType().Name} - {ex.Message}");
        }

        Console.WriteLine();

        // Exit with failure if HMAC-MD5 was successful
        if (hmacMd5Succeeded)
        {
            Console.WriteLine("ERROR: HMAC-MD5 calculation succeeded when it should have failed!");
            Console.WriteLine("Exiting with failure status.");
            return 1;
        }
        else
        {
            Console.WriteLine("SUCCESS: HMAC-MD5 is properly restricted.");
            return 0;
        }
    }
}
