<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Read mirror of Kendo time entries (ADR-0007), separate from the
        // `worklogs` outbox. Kendo ids only, no local FKs — like the issues
        // mirror. Billability is derived (projects.billable), never stored here.
        Schema::create('synced_worklogs', function (Blueprint $table) {
            $table->id();
            $table->unsignedBigInteger('kendo_worklog_id')->unique();
            $table->unsignedBigInteger('kendo_issue_id')->nullable();
            $table->unsignedBigInteger('kendo_project_id')->nullable();
            $table->unsignedInteger('minutes');
            $table->timestamp('started_at');
            $table->text('note')->nullable();
            $table->string('issue_key')->nullable();
            $table->string('issue_title')->nullable();
            $table->timestamp('synced_at')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('synced_worklogs');
    }
};
