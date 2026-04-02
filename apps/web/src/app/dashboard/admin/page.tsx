"use client";

import { useState, useEffect } from "react";
import { getVersionConfigs, getAdminUsers, updateVersionConfig, updateAdminUser, getAdminUserTunnels, bulkUpdateMinVersion } from "@/lib/actions/admin.actions";
import { VersionConfig, AdminUser, AdminTunnel, UpdateVersionConfigRequest } from "@/lib/actions/admin.types";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RefreshCw, Settings, Users, Package, Save, UserCheck, UserX, ChevronDown, ChevronRight, Globe, Zap } from "lucide-react";

export default function AdminPage() {
  const [configs, setConfigs] = useState<VersionConfig[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [usersLoading, setUsersLoading] = useState(true);
  const [editingConfig, setEditingConfig] = useState<VersionConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [userPage, setUserPage] = useState(1);
  const [totalUsers, setTotalUsers] = useState(0);

  // User tunnels expand state
  const [expandedUser, setExpandedUser] = useState<number | null>(null);
  const [userTunnels, setUserTunnels] = useState<AdminTunnel[]>([]);
  const [tunnelsLoading, setTunnelsLoading] = useState(false);

  // Bulk min version state
  const [bulkMinVersion, setBulkMinVersion] = useState("");
  const [bulkUpdating, setBulkUpdating] = useState(false);

  // Filtering and Sorting State
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [sortBy, setSortBy] = useState("id");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search);
      setUserPage(1); // Reset page on search change
    }, 400);
    return () => clearTimeout(timer);
  }, [search]);

  const fetchConfigs = async () => {
    try {
      setLoading(true);
      const data = await getVersionConfigs();
      setConfigs(data.configs || []);
    } catch (error) {
      console.error("Failed to fetch configs:", error);
    } finally {
      setLoading(false);
    }
  };

  const fetchUsers = async () => {
    try {
      setUsersLoading(true);
      const data = await getAdminUsers(userPage, 10, debouncedSearch, sortBy, sortOrder);
      setUsers(data.users || []);
      setTotalUsers(data.total_count || 0);
    } catch (error) {
      console.error("Failed to fetch users:", error);
    } finally {
      setUsersLoading(false);
    }
  };

  useEffect(() => {
    fetchConfigs();
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [userPage, debouncedSearch, sortBy, sortOrder]);

  const handleSaveConfig = async () => {
    if (!editingConfig) return;

    try {
      setSaving(true);
      const updateData: UpdateVersionConfigRequest = {
        channel: editingConfig.channel,
        platform: editingConfig.platform,
        arch: editingConfig.arch,
        latest_version: editingConfig.latest_version,
        minimum_version: editingConfig.minimum_version,
        download_url: editingConfig.download_url,
        release_notes: editingConfig.release_notes,
        auto_update_enabled: editingConfig.auto_update_enabled,
        force_update: editingConfig.force_update,
      };
      await updateVersionConfig(updateData);
      await fetchConfigs();
      setEditingConfig(null);
    } catch (error) {
      console.error("Failed to save config:", error);
    } finally {
      setSaving(false);
    }
  };

  const handleToggleUserActive = async (user: AdminUser) => {
    try {
      await updateAdminUser(user.id, { is_active: !user.is_active });
      await fetchUsers();
    } catch (error) {
      console.error("Failed to toggle user:", error);
    }
  };

  const handleExpandUser = async (userId: number) => {
    if (expandedUser === userId) {
      setExpandedUser(null);
      return;
    }
    setExpandedUser(userId);
    setTunnelsLoading(true);
    try {
      const tunnels = await getAdminUserTunnels(userId);
      setUserTunnels(tunnels || []);
    } catch (error) {
      console.error("Failed to fetch user tunnels:", error);
      setUserTunnels([]);
    } finally {
      setTunnelsLoading(false);
    }
  };

  const handleBulkUpdateMinVersion = async () => {
    if (!bulkMinVersion) return;
    setBulkUpdating(true);
    try {
      await bulkUpdateMinVersion({ minimum_version: bulkMinVersion });
      await fetchConfigs();
      setBulkMinVersion("");
    } catch (error) {
      console.error("Failed to bulk update min version:", error);
    } finally {
      setBulkUpdating(false);
    }
  };

  return (
    <Tabs defaultValue="versions" className="space-y-4">
      <TabsList>
        <TabsTrigger value="versions" className="flex items-center gap-2">
          <Package className="h-4 w-4" />
          Version Configs
        </TabsTrigger>
        <TabsTrigger value="users" className="flex items-center gap-2">
          <Users className="h-4 w-4" />
          Users
        </TabsTrigger>
      </TabsList>

      {/* Version Configs Tab */}
      <TabsContent value="versions" className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Client Version Configurations</h2>
          <Button variant="outline" size="sm" onClick={fetchConfigs} disabled={loading}>
            <RefreshCw className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        </div>

        <Card>
          <CardContent className="flex items-center gap-3 py-3">
            <Zap className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">Set Minimum Version for All:</span>
            <Input
              placeholder="e.g. v1.0.629"
              value={bulkMinVersion}
              onChange={(e) => setBulkMinVersion(e.target.value)}
              className="w-[160px]"
            />
            <Button
              size="sm"
              onClick={handleBulkUpdateMinVersion}
              disabled={!bulkMinVersion || bulkUpdating}
            >
              {bulkUpdating ? "Updating..." : "Apply to All"}
            </Button>
          </CardContent>
        </Card>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {configs.map((config) => (
            <Card key={config.id}>
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base capitalize">{config.channel}</CardTitle>
                  <Badge variant={config.force_update ? "destructive" : "secondary"}>
                    {config.force_update ? "Force Update" : "Optional"}
                  </Badge>
                </div>
                <CardDescription>
                  {config.platform} / {config.arch}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <span className="text-muted-foreground">Latest:</span>
                    <span className="ml-2 font-mono">{config.latest_version}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Minimum:</span>
                    <span className="ml-2 font-mono">{config.minimum_version}</span>
                  </div>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <Switch checked={config.auto_update_enabled} disabled />
                  <span className="text-muted-foreground">Auto-update</span>
                </div>
                <Dialog>
                  <DialogTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className="w-full"
                      onClick={() => setEditingConfig({ ...config })}
                    >
                      <Settings className="h-4 w-4 mr-2" />
                      Edit Config
                    </Button>
                  </DialogTrigger>
                  <DialogContent className="sm:max-w-[425px]">
                    <DialogHeader>
                      <DialogTitle>Edit Version Config</DialogTitle>
                      <DialogDescription>
                        Update version settings for {editingConfig?.channel} channel.
                      </DialogDescription>
                    </DialogHeader>
                    {editingConfig && (
                      <div className="grid gap-4 py-4">
                        <div className="grid grid-cols-4 items-center gap-4">
                          <Label htmlFor="min-version" className="text-right">
                            Minimum
                          </Label>
                          <Input
                            id="min-version"
                            value={editingConfig.minimum_version}
                            onChange={(e) =>
                              setEditingConfig({ ...editingConfig, minimum_version: e.target.value })
                            }
                            className="col-span-3"
                          />
                        </div>
                        <div className="grid grid-cols-4 items-center gap-4">
                          <Label htmlFor="latest-version" className="text-right">
                            Latest
                          </Label>
                          <Input
                            id="latest-version"
                            value={editingConfig.latest_version}
                            onChange={(e) =>
                              setEditingConfig({ ...editingConfig, latest_version: e.target.value })
                            }
                            className="col-span-3"
                          />
                        </div>
                        <div className="grid grid-cols-4 items-center gap-4">
                          <Label htmlFor="download-url" className="text-right">
                            Download
                          </Label>
                          <Input
                            id="download-url"
                            value={editingConfig.download_url}
                            onChange={(e) =>
                              setEditingConfig({ ...editingConfig, download_url: e.target.value })
                            }
                            className="col-span-3"
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <Label htmlFor="force-update">Force Update</Label>
                          <Switch
                            id="force-update"
                            checked={editingConfig.force_update}
                            onCheckedChange={(checked) =>
                              setEditingConfig({ ...editingConfig, force_update: checked })
                            }
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <Label htmlFor="auto-update">Auto Update</Label>
                          <Switch
                            id="auto-update"
                            checked={editingConfig.auto_update_enabled}
                            onCheckedChange={(checked) =>
                              setEditingConfig({ ...editingConfig, auto_update_enabled: checked })
                            }
                          />
                        </div>
                      </div>
                    )}
                    <DialogFooter>
                      <Button onClick={handleSaveConfig} disabled={saving}>
                        <Save className="h-4 w-4 mr-2" />
                        {saving ? "Saving..." : "Save Changes"}
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </CardContent>
            </Card>
          ))}
        </div>

        {configs.length === 0 && !loading && (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No version configurations found.
            </CardContent>
          </Card>
        )}
      </TabsContent>

      {/* Users Tab */}
      <TabsContent value="users" className="space-y-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <h2 className="text-xl font-semibold">User Management</h2>
          <div className="flex items-center gap-2">
            <Input
              placeholder="Search by email..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-[200px]"
            />
            <Button variant="outline" size="sm" onClick={fetchUsers} disabled={usersLoading}>
              <RefreshCw className={`h-4 w-4 ${usersLoading ? "animate-spin" : ""}`} />
              <span className="sr-only">Refresh</span>
            </Button>
          </div>
        </div>

        <div className="text-sm text-muted-foreground mb-2">
          {totalUsers} total users found
        </div>

        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="cursor-pointer hover:text-foreground" onClick={() => {
                  if (sortBy === "email") {
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortBy("email");
                    setSortOrder("asc");
                  }
                }}>
                  User {sortBy === "email" && (sortOrder === "asc" ? "↑" : "↓")}
                </TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="cursor-pointer hover:text-foreground" onClick={() => {
                  if (sortBy === "last_login") {
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortBy("last_login");
                    setSortOrder("desc");
                  }
                }}>
                  Last Login {sortBy === "last_login" && (sortOrder === "asc" ? "↑" : "↓")}
                </TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <>
                  <TableRow key={user.id} className="cursor-pointer" onClick={() => handleExpandUser(user.id)}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {expandedUser === user.id ? (
                          <ChevronDown className="h-4 w-4 text-muted-foreground" />
                        ) : (
                          <ChevronRight className="h-4 w-4 text-muted-foreground" />
                        )}
                        <div>
                          <div className="font-medium">{user.name || "No name"}</div>
                          <div className="text-sm text-muted-foreground">{user.email}</div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant={user.role === "admin" ? "default" : "secondary"}>
                        {user.role}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={user.is_active ? "default" : "destructive"}>
                        {user.is_active ? "Active" : "Inactive"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {user.last_login
                        ? new Date(user.last_login).toLocaleDateString()
                        : "Never"}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => { e.stopPropagation(); handleToggleUserActive(user); }}
                      >
                        {user.is_active ? (
                          <UserX className="h-4 w-4 text-red-500" />
                        ) : (
                          <UserCheck className="h-4 w-4 text-green-500" />
                        )}
                      </Button>
                    </TableCell>
                  </TableRow>
                  {expandedUser === user.id && (
                    <TableRow key={`${user.id}-tunnels`}>
                      <TableCell colSpan={5} className="bg-muted/50 p-4">
                        {tunnelsLoading ? (
                          <div className="text-sm text-muted-foreground">Loading tunnels...</div>
                        ) : userTunnels.length === 0 ? (
                          <div className="text-sm text-muted-foreground">No tunnels configured</div>
                        ) : (
                          <div className="space-y-2">
                            <div className="text-sm font-medium mb-2">Tunnels ({userTunnels.length})</div>
                            <div className="grid gap-2">
                              {userTunnels.map((t) => (
                                <div key={t.id} className="flex items-center gap-4 text-sm bg-background rounded-md px-3 py-2 border">
                                  <Globe className="h-4 w-4 text-muted-foreground shrink-0" />
                                  <span className="font-mono font-medium">{t.domain}</span>
                                  <span className="text-muted-foreground">→</span>
                                  <span className="font-mono">{t.target_host === "localhost" ? "" : t.target_host + ":"}{t.target_port}</span>
                                  <Badge variant={t.is_enabled ? "default" : "secondary"} className="ml-auto">
                                    {t.is_enabled ? "Enabled" : "Disabled"}
                                  </Badge>
                                  {t.client_ip && (
                                    <Badge variant="outline" className="text-green-600">
                                      Connected ({t.client_ip})
                                    </Badge>
                                  )}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </TableCell>
                    </TableRow>
                  )}
                </>
              ))}
            </TableBody>
          </Table>
        </Card>

        {users.length === 0 && !usersLoading && (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No users found.
            </CardContent>
          </Card>
        )}

        {/* Pagination */}
        {totalUsers > 10 && (
          <div className="flex items-center justify-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setUserPage((p) => Math.max(1, p - 1))}
              disabled={userPage === 1}
            >
              Previous
            </Button>
            <span className="text-sm text-muted-foreground">
              Page {userPage} of {Math.ceil(totalUsers / 10)}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setUserPage((p) => p + 1)}
              disabled={userPage >= Math.ceil(totalUsers / 10)}
            >
              Next
            </Button>
          </div>
        )}
      </TabsContent>
    </Tabs>
  );
}
