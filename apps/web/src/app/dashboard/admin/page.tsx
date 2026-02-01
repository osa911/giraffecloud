"use client";

import { useState, useEffect } from "react";
import { getVersionConfigs, getAdminUsers, updateVersionConfig, updateAdminUser } from "@/lib/actions/admin.actions";
import { VersionConfig, AdminUser, UpdateVersionConfigRequest } from "@/lib/actions/admin.types";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { RefreshCw, Settings, Users, Package, Save, UserCheck, UserX } from "lucide-react";

export default function AdminPage() {
  const [configs, setConfigs] = useState<VersionConfig[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [usersLoading, setUsersLoading] = useState(true);
  const [editingConfig, setEditingConfig] = useState<VersionConfig | null>(null);
  const [saving, setSaving] = useState(false);
  const [userPage, setUserPage] = useState(1);
  const [totalUsers, setTotalUsers] = useState(0);

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
      const data = await getAdminUsers(userPage, 10);
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
    fetchUsers();
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [userPage]);

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
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">User Management</h2>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">
              {totalUsers} total users
            </span>
            <Button variant="outline" size="sm" onClick={fetchUsers} disabled={usersLoading}>
              <RefreshCw className={`h-4 w-4 mr-2 ${usersLoading ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
        </div>

        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Login</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell>
                    <div>
                      <div className="font-medium">{user.name || "No name"}</div>
                      <div className="text-sm text-muted-foreground">{user.email}</div>
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
                      onClick={() => handleToggleUserActive(user)}
                    >
                      {user.is_active ? (
                        <UserX className="h-4 w-4 text-red-500" />
                      ) : (
                        <UserCheck className="h-4 w-4 text-green-500" />
                      )}
                    </Button>
                  </TableCell>
                </TableRow>
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
