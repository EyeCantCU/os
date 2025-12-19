import java.lang.management.ManagementFactory;
import javax.management.*;

public class SimpleApp {
    public static void main(String[] args) throws Exception {
        MBeanServer mbs = ManagementFactory.getPlatformMBeanServer();
        ObjectName name = new ObjectName("com.example:type=Simple");
        mbs.registerMBean(new Simple(), name);
        System.out.println("SimpleApp running...");
        Thread.sleep(Long.MAX_VALUE);
    }

    public interface SimpleMBean {
        int getValue();
        void setValue(int value);
    }

    public static class Simple implements SimpleMBean {
        private int value = 42;

        public int getValue() { return value; }
        public void setValue(int value) { this.value = value; }
    }
}
