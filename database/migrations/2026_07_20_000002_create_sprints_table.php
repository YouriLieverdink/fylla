<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Minimal sprint mirror (issue #56): just enough to render the Client brief's
     * current-sprint tile — name, dates, status. Mirrored per project inside
     * SyncKendoProjectIssues; done/total is a group-by on synced_issues.sprint_id,
     * not stored here. Carries no Fylla-owned fields, so a plain reconcile-delete
     * is safe (like synced_issues).
     */
    public function up(): void
    {
        Schema::create('sprints', function (Blueprint $table) {
            $table->id();
            $table->unsignedBigInteger('kendo_id')->unique();
            $table->unsignedBigInteger('project_id')->nullable();
            $table->string('name')->nullable();
            // Kendo sprint status: 1 = active (the only value we key on).
            $table->integer('status')->nullable();
            $table->timestamp('starts_at')->nullable();
            $table->timestamp('ends_at')->nullable();
            $table->timestamp('synced_at')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('sprints');
    }
};
