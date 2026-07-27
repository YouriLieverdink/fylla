<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/** Superseded by job_runs.error (#91) — the failure was recorded twice. */
return new class extends Migration
{
    public function up(): void
    {
        Schema::table('worklogs', function (Blueprint $table) {
            $table->dropColumn('post_error');
        });
    }

    public function down(): void
    {
        Schema::table('worklogs', function (Blueprint $table) {
            $table->text('post_error')->nullable();
        });
    }
};
