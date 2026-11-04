public class BulkFailure {
    public static Integer run(String cohort, String newName) {
        String firstGroup = 'first';
        List<Account> first = [SELECT Id, Name, Probe_Group__c FROM Account WHERE Probe_Group__c = :firstGroup];
        for (Account a : first) { a.Name = newName; }
        update first;
        List<Account> rows = [SELECT Id, Name, Probe_Group__c FROM Account WHERE Probe_Group__c = :cohort];
        for (Account a : rows) { a.Name = newName; }
        update rows;
        return 0;
    }
}
