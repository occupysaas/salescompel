public class Compute {
    public static Integer run(Integer n) {
        Integer total = 0;
        Integer i = 0;
        while (i < n) {
            total = total + i;
            i = i + 1;
        }
        if (total > 0) { return total; } else { return 0; }
    }
}
