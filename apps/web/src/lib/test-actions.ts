import {isWithinInterval, parseISO} from 'date-fns';
import _ from 'lodash';

/**
 * Interface defining a maintenance window configuration
 */
interface MaintenanceWindow {
  start: Date;
  end: Date;
  maintenanceTlds?: DomainSuffixes[];
  disableIdentityDigitalTlds?: boolean;
  disableGoDaddyTlds?: boolean;
  disablePublicInterestRegistryTlds?: boolean;
  disableNominetTlds?: boolean;
}

/**
 * UPDATE THIS SECTION TO SCHEDULE MAINTENANCE WINDOWS
 * Add maintenance windows in chronological order
 */
const MAINTENANCE_WINDOWS: MaintenanceWindow[] = [
  // 2025-12-14 0100 - 0145 UTC: Verisign (.com, .net)
  {
    start: parseISO('2025-12-14T01:00:00Z'),
    end: parseISO('2025-12-14T01:45:00Z'),
    maintenanceTlds: ['com', 'net'],
  },
  // Add more maintenance windows as needed
];

const addFlagOverrides = (): string[] | null => {
    const now = new Date();
  const overrides = {};

  // Check if we're currently in any maintenance window
  const activeWindow = MAINTENANCE_WINDOWS.find(({start, end}) =>
    isWithinInterval(now, {start, end}),
  );

  if (activeWindow) {
    return activeWindow.maintenanceTlds
  }

  return null
  
}

export default addFlagOverrides;
