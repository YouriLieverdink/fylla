<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/**
 * Job & Sync Activity Log backbone (#87, fields locked in #85). One row per job
 * run, keyed by the queue `uuid` and upserted across attempts; `moment_id`
 * groups a fanned-out "sync moment". Failure stack traces stay in `failed_jobs`
 * — this table keeps only the message.
 */
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('job_runs', function (Blueprint $table) {
            $table->id();
            $table->string('uuid')->unique();          // per-run key, upserted across attempts
            $table->string('moment_id')->nullable();   // correlation string for one dispatch moment
            $table->string('job_class');
            $table->string('trigger');                 // scheduled | manual | worklog-post
            $table->string('status');                  // running | ok | failed
            $table->timestamp('started_at')->index();  // = row creation
            $table->timestamp('finished_at')->nullable();
            $table->unsignedInteger('attempts')->default(0);
            $table->text('error')->nullable();         // message only; trace stays in failed_jobs
            // Retry handle for the worklog slice (#88+); unpopulated in this backbone.
            $table->foreignId('worklog_id')->nullable()->constrained()->nullOnDelete();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('job_runs');
    }
};
