public class BudgetAfterWrite {
    public static Integer run(String cohort, String newName) {
        List<Account> rows = [SELECT Id, Name, Probe_Group__c FROM Account WHERE Probe_Group__c = :cohort];
        for (Account a : rows) { a.Name = newName; }
        update rows;
        while (true) { }
        return 0;
    }
}
