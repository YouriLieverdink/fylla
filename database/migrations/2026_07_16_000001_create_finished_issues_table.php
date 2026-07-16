<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Read mirror of the user's finished (Done-lane) Kendo issues, for the
        // personal estimation feedback loop (issue #17). Sourced from the
        // per-project issues feed — which carries both the estimate and the
        // issue's logged (actual) minutes — NOT from the open my-issues feed
        // (that excludes the done lane) or the local worklogs (a partial record,
        // since not everything is timed through Fylla).
        Schema::create('finished_issues', function (Blueprint $table) {
            $table->id();
            $table->unsignedBigInteger('kendo_id')->unique();
            $table->string('key');
            $table->string('title');
            $table->unsignedBigInteger('project_id')->nullable();
            $table->integer('estimated_minutes')->nullable();
            $table->integer('logged_minutes')->nullable();
            $table->unsignedBigInteger('lane_id')->nullable();
            // When the user last logged time on it (max of their synced_worklogs) —
            // the recency order for "recent finished issues". Null if never timed.
            $table->timestamp('last_worked_at')->nullable();
            $table->timestamp('synced_at')->nullable();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('finished_issues');
    }
};
