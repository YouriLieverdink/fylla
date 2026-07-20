<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Generalize the personal finished-issue mirror into a team-wide issue mirror
     * (issue #55): every assignee, every lane — the substrate the Client context
     * page reads (#56). The personal estimation loop (#17) keeps reading it
     * filtered to the user's own done issues.
     */
    public function up(): void
    {
        Schema::rename('finished_issues', 'synced_issues');

        Schema::table('synced_issues', function (Blueprint $table) {
            // The "who" — joins to developers.kendo_id (R2: unified id space).
            $table->unsignedBigInteger('assignee_id')->nullable()->index();
            // 'first' | 'middle' | 'done', classified at sync time from the lane
            // order — encodes the three-way split so reports stay pure SQL.
            $table->string('lane_position')->nullable();
            // The lane's display title (e.g. "In review") — for the aging panel (#56).
            $table->string('lane_name')->nullable();
            // Fylla-recorded lane-entry time (Kendo exposes none, R1) — forward-only.
            $table->timestamp('lane_entered_at')->nullable();
            // Active-sprint membership, for the Client brief (#56 minimal sprint sync).
            $table->unsignedBigInteger('sprint_id')->nullable();
        });
    }

    public function down(): void
    {
        Schema::table('synced_issues', function (Blueprint $table) {
            $table->dropColumn(['assignee_id', 'lane_position', 'lane_name', 'lane_entered_at', 'sprint_id']);
        });

        Schema::rename('synced_issues', 'finished_issues');
    }
};
